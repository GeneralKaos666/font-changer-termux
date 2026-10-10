package tui

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/GeneralKaos666/nerdfont-changer/internal/apply"
	"github.com/GeneralKaos666/nerdfont-changer/internal/downloader"
	"github.com/GeneralKaos666/nerdfont-changer/internal/paths"
	"github.com/GeneralKaos666/nerdfont-changer/internal/scan"
)

func useTermuxHome(t *testing.T) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), "termux")
	t.Setenv("TERMUX_HOME", root)
	return root
}

// noReload forces ReloadSettings down the missing-binary path so tests are
// hermetic on machines that do (or do not) ship termux-reload-settings.
func noReload(t *testing.T) {
	t.Helper()
	t.Setenv("PATH", t.TempDir())
}

func fixtureBytes(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "apply", "testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// seedLibrary copies the valid apply fixtures into the font library under
// the given names.
func seedLibrary(t *testing.T, names ...string) {
	t.Helper()
	dir := paths.FontsDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	fixtures := []string{"a.ttf", "b.ttf"}
	for i, name := range names {
		if err := os.WriteFile(filepath.Join(dir, name), fixtureBytes(t, fixtures[i%len(fixtures)]), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func updateModel(t *testing.T, m Model, msg tea.Msg) Model {
	t.Helper()
	next, _ := m.Update(msg)
	out, ok := next.(Model)
	if !ok {
		t.Fatalf("Update(%T) returned %T, want tui.Model", msg, next)
	}
	return out
}

func keyRunes(s string) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}

func TestModel_CycleSlot(t *testing.T) {
	useTermuxHome(t)
	noReload(t)
	seedLibrary(t, "Hack.ttf")

	m := NewModel()
	if m.slot != "regular" {
		t.Fatalf("initial slot = %q, want regular", m.slot)
	}
	m = updateModel(t, m, keyRunes("s"))
	if m.slot != "bold" {
		t.Fatalf("slot after one s = %q, want bold", m.slot)
	}
	// Four presses is a full cycle through the four slots.
	for i := 0; i < 3; i++ {
		m = updateModel(t, m, keyRunes("s"))
	}
	if m.slot != "regular" {
		t.Fatalf("slot after four s presses = %q, want regular", m.slot)
	}
}

func TestModel_FilterNarrowsList(t *testing.T) {
	useTermuxHome(t)
	noReload(t)
	seedLibrary(t, "Hack.ttf", "JetBrainsMono.ttf", "FiraCode.ttf")

	m := NewModel()
	if got := len(m.VisibleEntries()); got != 3 {
		t.Fatalf("unfiltered visible entries = %d, want 3", got)
	}

	m = updateModel(t, m, FilterMsg("hack"))
	visible := m.VisibleEntries()
	if len(visible) != 1 {
		t.Fatalf("filtered visible entries = %d, want 1", len(visible))
	}
	if visible[0].Name != "Hack.ttf" {
		t.Fatalf("filtered entry = %q, want Hack.ttf", visible[0].Name)
	}
}

func TestModel_HighlightIsSideEffectFree(t *testing.T) {
	useTermuxHome(t)
	noReload(t)
	seedLibrary(t, "Hack.ttf", "JetBrainsMono.ttf", "FiraCode.ttf")

	target, err := paths.FontSlotPath("regular")
	if err != nil {
		t.Fatal(err)
	}
	original := fixtureBytes(t, "a.ttf")
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, original, 0o644); err != nil {
		t.Fatal(err)
	}

	m := NewModel()
	m = updateModel(t, m, tea.KeyMsg{Type: tea.KeyDown})
	m = updateModel(t, m, tea.KeyMsg{Type: tea.KeyDown})
	m = updateModel(t, m, tea.KeyMsg{Type: tea.KeyUp})

	if m.Dirty() {
		t.Fatal("cursor move marked preview dirty")
	}
	after, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(original) {
		t.Fatal("cursor move changed the slot file")
	}
}

func TestPreviewView_ShowsSampleAndCoverage(t *testing.T) {
	useTermuxHome(t)
	noReload(t)
	seedLibrary(t, "Hack.ttf", "JetBrainsMono.ttf")

	m := NewModel()
	pane := m.PreviewPane()
	for _, want := range []string{"AaBbCc 0123456789", "\ue0a0", "font.ttf", "backup"} {
		if !strings.Contains(pane, want) {
			t.Fatalf("preview pane missing %q:\n%s", want, pane)
		}
	}
}

func TestDownload_ShowsProgress(t *testing.T) {
	useTermuxHome(t)
	noReload(t)

	m := NewModel()
	m = updateModel(t, m, downloadStartMsg{name: "Hack-Regular"})
	if !m.Downloading() {
		t.Fatal("downloadStartMsg did not mark download active")
	}

	m = updateModel(t, m, downloadProgressMsg{frac: 0.5, gen: m.dlGen, known: true})
	if got := m.DownloadProgress(); got != 0.5 {
		t.Fatalf("download progress = %v, want 0.5", got)
	}
	if strings.Contains(m.status, "no progress info") {
		t.Fatalf("status after measured progress = %q, want the indeterminate wording dropped", m.status)
	}
	if !m.Downloading() {
		t.Fatal("download dismissed before done")
	}

	m = updateModel(t, m, downloadDoneMsg{path: "/tmp/Hack.ttf", gen: m.dlGen})
	if m.Downloading() {
		t.Fatal("downloadDoneMsg did not dismiss the download")
	}
	if got := m.DownloadProgress(); got != 0 {
		t.Fatalf("progress after done = %v, want 0", got)
	}
}

// TestDownload_StaleProgressIgnored pins generation filtering: progress
// from a dismissed or superseded fetch must not move the bar or touch
// the status line.
func TestDownload_StaleProgressIgnored(t *testing.T) {
	useTermuxHome(t)
	noReload(t)

	m := NewModel()
	m = updateModel(t, m, downloadStartMsg{name: "Hack-Regular"})
	if !m.Downloading() {
		t.Fatal("downloadStartMsg did not mark download active")
	}
	status := m.status

	m = updateModel(t, m, downloadProgressMsg{frac: 0.7, gen: m.dlGen - 1, known: true})
	if got := m.DownloadProgress(); got != 0 {
		t.Fatalf("stale progress moved the bar to %v, want 0", got)
	}
	if m.status != status {
		t.Fatalf("stale progress rewrote status: %q → %q", status, m.status)
	}
}

// TestDownload_PollReArm pins the poll pipeline: downloadStartMsg hands
// back the first channel read, and every current progress message arms
// the next read. The cmds are never executed — running one would block
// on (or trigger) the real fetch and break hermiticity.
func TestDownload_PollReArm(t *testing.T) {
	useTermuxHome(t)
	noReload(t)

	m := NewModel()
	next, cmd := m.Update(downloadStartMsg{name: "Hack-Regular"})
	m = next.(Model)
	if cmd == nil {
		t.Fatal("downloadStartMsg returned no cmd (channel read missing)")
	}
	next, cmd = m.Update(downloadProgressMsg{frac: 0.5, gen: m.dlGen, known: true})
	m = next.(Model)
	if cmd == nil {
		t.Fatal("progress message returned no re-arm cmd (poll would stop)")
	}
}

// TestDownload_DismissReleasesFetchChannel pins that cancelling a live
// download detaches the pipeline, so the producer is not left blocked.
func TestDownload_DismissReleasesFetchChannel(t *testing.T) {
	useTermuxHome(t)
	noReload(t)

	m := NewModel()
	m.overlay = overlayDownload
	m = updateModel(t, m, downloadStartMsg{name: "Hack-Regular"})
	if m.dlCh == nil {
		t.Fatal("start did not attach a fetch channel")
	}
	m = updateModel(t, m, tea.KeyMsg{Type: tea.KeyEsc})
	if m.dlCh != nil {
		t.Fatal("dismiss left the fetch channel attached")
	}
}

func TestNewModel_SurfacesLibraryLoadError(t *testing.T) {
	useTermuxHome(t)
	noReload(t)

	old := listLibrary
	listLibrary = func() ([]scan.FontEntry, error) {
		return nil, errors.New("boom")
	}
	defer func() { listLibrary = old }()

	m := NewModel()
	if !strings.Contains(m.status, "boom") {
		t.Fatalf("load-error status = %q, want it to mention the error", m.status)
	}
	if len(m.VisibleEntries()) != 0 {
		t.Fatalf("visible entries = %d, want 0 on load failure", len(m.VisibleEntries()))
	}
}

func TestQuit_RestoreFailureStillQuits(t *testing.T) {
	useTermuxHome(t)
	noReload(t)
	seedLibrary(t, "Hack.ttf")

	m := NewModel()
	m = updateModel(t, m, keyRunes(" ")) // preview → dirty
	old := restoreOriginal
	restoreOriginal = func(*apply.SessionState) (bool, error) { return false, errors.New("boom") }
	defer func() { restoreOriginal = old }()

	next, cmd := m.Update(keyRunes("q")) // q with failed restore
	m = next.(Model)
	if cmd == nil {
		t.Fatal("q with failed restore must still quit")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatalf("q cmd returned %T, want tea.QuitMsg", cmd())
	}
	if !strings.Contains(m.status, "Restore failed") {
		t.Fatalf("status = %q, want restore failure text", m.status)
	}

	restoreOriginal = old
	_, cmd = m.Update(keyRunes("q"))
	if cmd == nil {
		t.Fatal("q with successful restore returned no quit cmd")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatalf("q cmd returned %T, want tea.QuitMsg", cmd())
	}
}

func TestDownload_ShowsIndeterminateStatus(t *testing.T) {
	useTermuxHome(t)
	noReload(t)

	m := NewModel()
	m = updateModel(t, m, downloadStartMsg{name: "Hack-Regular"})
	if !m.Downloading() {
		t.Fatal("downloadStartMsg did not mark download active")
	}
	if !strings.Contains(m.status, "no progress info") {
		t.Fatalf("download status = %q, want indeterminate wording", m.status)
	}
	if got := m.View(); !strings.Contains(got, "no progress info") {
		t.Fatal("download view does not say indeterminate")
	}
}

func TestDownload_DismissIgnoresLateDone(t *testing.T) {
	useTermuxHome(t)
	noReload(t)
	seedLibrary(t, "Hack.ttf")

	m := NewModel()
	m.overlay = overlayDownload
	m = updateModel(t, m, downloadStartMsg{name: "Hack-Regular"})
	if !m.Downloading() {
		t.Fatal("download did not start")
	}
	m = updateModel(t, m, tea.KeyMsg{Type: tea.KeyEsc})
	if m.Downloading() {
		t.Fatal("esc did not dismiss active download")
	}
	if got := m.status; strings.Contains(got, "cancelled") {
		t.Fatalf("dismiss overclaims cancellation: %q", got)
	}
	// Late completion of the orphaned fetch must not report a download.
	m = updateModel(t, m, downloadDoneMsg{path: "/tmp/Hack.ttf", gen: m.dlGen - 1})
	if got := m.status; strings.Contains(got, "Downloaded") {
		t.Fatalf("stale done overwrote status: %q", got)
	}
}

func TestPreviewPane_ShowsDetailsAndPrompt(t *testing.T) {
	useTermuxHome(t)
	noReload(t)
	seedLibrary(t, "Hack.ttf")

	m := NewModel()
	pane := m.PreviewPane()
	for _, want := range []string{"glyphs", "UPM", "❯"} {
		if !strings.Contains(pane, want) {
			t.Fatalf("preview pane missing %q:\n%s", want, pane)
		}
	}
}

func TestView_VerticalLayout(t *testing.T) {
	useTermuxHome(t)
	noReload(t)
	seedLibrary(t, "Alpha.ttf", "Beta.ttf")

	m := NewModel()
	m.width, m.height = 100, 40
	m.sizeWidgets()
	view := m.View()
	// The panes are stacked, never side by side: no line carries two
	// top-left box corners.
	if hasSideBySideBoxes(view) {
		t.Fatalf("expected a vertical stack, got two columns:\n%s", view)
	}
	// The preview sits above the library.
	pane := strings.Index(view, "╭─ Preview")
	lib := strings.Index(view, "╭─ Library")
	if pane < 0 || lib < 0 {
		t.Fatalf("missing titled panes (preview=%d library=%d):\n%s", pane, lib, view)
	}
	if pane > lib {
		t.Fatalf("preview should sit above the library:\n%s", view)
	}
	// Both panes render their content.
	entries := m.VisibleEntries()
	if len(entries) < 2 {
		t.Fatalf("want >= 2 visible entries, got %d", len(entries))
	}
	if !strings.Contains(view, entries[1].Name) {
		t.Fatalf("library list missing %q:\n%s", entries[1].Name, view)
	}
	if !strings.Contains(view, "AaBbCc") {
		t.Fatalf("preview missing sample:\n%s", view)
	}
}

// hasSideBySideBoxes reports whether any line contains two top-left box
// corners, i.e. at least two bordered boxes rendered on one row.
func hasSideBySideBoxes(view string) bool {
	for _, line := range strings.Split(view, "\n") {
		if strings.Count(line, "╭") >= 2 {
			return true
		}
	}
	return false
}

// TestLayout_PaneLinesFillWidth pins the titled border to the body width:
// the box glyphs are multibyte, so a byte-length slip shrinks the top rule.
func TestLayout_PaneLinesFillWidth(t *testing.T) {
	useTermuxHome(t)
	noReload(t)
	seedLibrary(t, "Alpha.ttf", "Beta.ttf")

	m := NewModel()
	m.width, m.height = 100, 40
	m.sizeWidgets()
	for _, line := range strings.Split(m.View(), "\n") {
		if !strings.HasPrefix(line, "╭") && !strings.HasPrefix(line, "│") && !strings.HasPrefix(line, "╰") {
			continue
		}
		if w := lipgloss.Width(line); w != 100 {
			t.Fatalf("pane line is %d cells wide, want 100: %q", w, line)
		}
	}
}

func TestDownload_FilterNarrowsList(t *testing.T) {
	useTermuxHome(t)
	noReload(t)

	m := NewModel()
	m = updateModel(t, m, keyRunes("d"))
	if len(m.dlFiltered) < 46 {
		t.Fatalf("download catalog = %d, want the full family list", len(m.dlFiltered))
	}
	m = updateModel(t, m, tea.KeyMsg{Type: tea.KeyTab})
	if !m.dlFilterFocused {
		t.Fatal("tab did not focus the download filter")
	}
	m = updateModel(t, m, keyRunes("fira"))
	if len(m.dlFiltered) == 0 || len(m.dlFiltered) >= len(m.dlNames) {
		t.Fatalf("filter \"fira\" kept %d of %d names", len(m.dlFiltered), len(m.dlNames))
	}
	for _, n := range m.dlFiltered {
		if !strings.Contains(strings.ToLower(n), "fira") {
			t.Fatalf("filtered list kept non-matching name %q", n)
		}
	}
	if m.dlCursor != 0 {
		t.Fatalf("cursor = %d, want 0 after narrowing", m.dlCursor)
	}
}

func TestDownload_WindowKeepsCursorVisible(t *testing.T) {
	useTermuxHome(t)
	noReload(t)

	m := NewModel()
	m = updateModel(t, m, keyRunes("d"))
	if len(m.dlFiltered) <= dlWindow {
		t.Fatalf("need more than %d catalog entries to test the window, got %d", dlWindow, len(m.dlFiltered))
	}
	assertVisible := func(t *testing.T) {
		t.Helper()
		if m.dlOffset > m.dlCursor || m.dlCursor >= m.dlOffset+dlWindow {
			t.Fatalf("cursor %d outside window [%d,%d)", m.dlCursor, m.dlOffset, m.dlOffset+dlWindow)
		}
	}
	for i := 0; i < len(m.dlFiltered)+5; i++ {
		m = updateModel(t, m, tea.KeyMsg{Type: tea.KeyDown})
		assertVisible(t)
	}
	if want := len(m.dlFiltered) - 1; m.dlCursor != want {
		t.Fatalf("cursor clamped at %d, want last index %d", m.dlCursor, want)
	}
	for i := 0; i < len(m.dlFiltered)+5; i++ {
		m = updateModel(t, m, tea.KeyMsg{Type: tea.KeyUp})
		assertVisible(t)
	}
	if m.dlCursor != 0 || m.dlOffset != 0 {
		t.Fatalf("cursor/offset at top = %d/%d, want 0/0", m.dlCursor, m.dlOffset)
	}
}

func TestDownload_LayoutFitsAndStacks(t *testing.T) {
	useTermuxHome(t)
	noReload(t)
	seedLibrary(t, "Alpha.ttf", "Beta.ttf")

	m := NewModel()
	m.width, m.height = 100, 40
	m.sizeWidgets()
	m = updateModel(t, m, keyRunes("d"))
	view := m.View()
	if h := lipgloss.Height(view); h != 40 {
		t.Fatalf("download view height = %d, want 40", h)
	}
	if !strings.Contains(view, "Nerd Fonts") {
		t.Fatalf("download box missing:\n%s", view)
	}
	// The picker swaps into the lower pane; the preview stays on top and
	// nothing renders side by side.
	if !strings.Contains(view, "╭─ Preview") {
		t.Fatalf("download view dropped the preview:\n%s", view)
	}
	if hasSideBySideBoxes(view) {
		t.Fatalf("download view should stack, not sit side by side:\n%s", view)
	}
}

func TestList_ShowsAppliedBadge(t *testing.T) {
	useTermuxHome(t)
	noReload(t)
	seedLibrary(t, "Hack.ttf")
	// Make the regular slot byte-identical to the library font.
	src, err := os.ReadFile(filepath.Join(paths.FontsDir(), "Hack.ttf"))
	if err != nil {
		t.Fatal(err)
	}
	slot, err := paths.FontSlotPath("regular")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(slot, src, 0o644); err != nil {
		t.Fatal(err)
	}

	m := NewModel()
	view := m.View()
	if !strings.Contains(view, "● regular") {
		t.Fatalf("list missing applied badge:\n%s", view)
	}
}

func TestTheme_MissingPaletteKeepsDefaults(t *testing.T) {
	useTermuxHome(t)
	noReload(t)
	seedLibrary(t, "Hack.ttf")

	// No colors.properties in TERMUX_HOME → defaults, no crash.
	m := NewModel()
	if m.status != "" && strings.Contains(m.status, "Theme") {
		t.Fatalf("unexpected theme status: %q", m.status)
	}
	view := m.View()
	if !strings.Contains(view, "Hack.ttf") {
		t.Fatal("view broken without palette")
	}
}

func TestPrompt_FallbackOnBadShell(t *testing.T) {
	useTermuxHome(t)
	noReload(t)
	seedLibrary(t, "Hack.ttf")
	t.Setenv("SHELL", "/nonexistent-shell-xyz")

	m := NewModel()
	pane := m.PreviewPane()
	if !strings.Contains(pane, "❯") {
		t.Fatalf("fallback mock prompt missing:\n%s", pane)
	}
}

func TestPrompt_CapturedByInitCommand(t *testing.T) {
	useTermuxHome(t)
	noReload(t)
	seedLibrary(t, "Hack.ttf")
	old := capturePromptLines
	capturePromptLines = func() []string { return []string{"\x1b[32m╭─ injected", "╰─❯ test"} }
	defer func() { capturePromptLines = old }()

	// NewModel must not block on the shell: the capture is deferred to the
	// Init command, so the first frame renders with the mock prompt.
	m := NewModel()
	if m.prompt != nil {
		t.Fatalf("NewModel captured the prompt synchronously: %q", m.prompt)
	}
	if pane := m.PreviewPane(); strings.Contains(pane, "injected") {
		t.Fatal("prompt appeared before the Init command ran")
	}

	cmd := m.Init()
	if cmd == nil {
		t.Fatal("Init returned no prompt-capture command")
	}
	m = updateModel(t, m, cmd())
	if pane := m.PreviewPane(); !strings.Contains(pane, "injected") {
		t.Fatalf("injected prompt missing:\n%s", pane)
	}
}

// Keep NewModel hermetic: no real shell capture in tests (per-test
// overrides still work by reassigning capturePromptLines), and no real
// network when a test drives downloadStartMsg — startFetch spawns its
// fetch goroutine eagerly (per-test overrides still work by reassigning
// fetchFont).
func TestMain(m *testing.M) {
	capturePromptLines = func() []string { return nil }
	fetchFont = func(string, bool, downloader.ProgressFunc) (string, error) {
		return filepath.Join(os.TempDir(), "stub-font.ttf"), nil
	}
	os.Exit(m.Run())
}

func TestTruncateCells_WidthAware(t *testing.T) {
	if got := truncateCells("AaBbCc", 4); got != "AaB…" {
		t.Fatalf("ascii: got %q", got)
	}
	if got := truncateCells("日本語ab", 4); got != "日…" {
		t.Fatalf("wide: got %q", got)
	}
	if got := truncateCells("hi", 10); got != "hi" {
		t.Fatalf("short: got %q", got)
	}
}

func TestCommit_ToastNamesCommittedSlot(t *testing.T) {
	useTermuxHome(t)
	noReload(t)
	seedLibrary(t, "Hack.ttf")

	m := NewModel()
	m = updateModel(t, m, keyRunes(" ")) // preview into slot "regular"
	m = updateModel(t, m, keyRunes("s")) // cycle slot to "bold"
	m = updateModel(t, m, keyRunes("enter"))
	if got := m.status; !strings.Contains(got, "Kept Hack.ttf → regular slot") {
		t.Fatalf("commit status = %q, want it to name the committed regular slot", got)
	}
	if strings.Contains(m.status, "bold slot") {
		t.Fatalf("commit status names the current slot instead of the committed one: %q", m.status)
	}
}

func TestCommit_InstallToastNamesFontAndSlot(t *testing.T) {
	useTermuxHome(t)
	noReload(t)
	seedLibrary(t, "Hack.ttf")

	m := NewModel()
	m = updateModel(t, m, keyRunes("enter")) // no preview → non-dirty install path
	if !strings.HasPrefix(m.status, "Installed ") {
		t.Fatalf("status = %q, want it to start with %q", m.status, "Installed ")
	}
	if !strings.Contains(m.status, "Hack.ttf") || !strings.Contains(m.status, "regular slot") {
		t.Fatalf("status = %q, want the font name and slot in the toast", m.status)
	}
}

func TestLayout_TilesTerminalHeight(t *testing.T) {
	useTermuxHome(t)
	noReload(t)
	seedLibrary(t, "Alpha.ttf", "Beta.ttf")

	m := NewModel()
	m.width, m.height = 100, 40
	m.sizeWidgets()
	view := m.View()
	if h := lipgloss.Height(view); h != 40 {
		t.Fatalf("view height = %d, want 40 (terminal height)", h)
	}
}

// stageFont writes the valid fixture to a temp path outside the library.
func stageFont(t *testing.T, name string) string {
	t.Helper()
	src := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(src, fixtureBytes(t, "a.ttf"), 0o644); err != nil {
		t.Fatal(err)
	}
	return src
}

// writeLibraryFont plants data directly in the library under name.
func writeLibraryFont(t *testing.T, name string, data []byte) string {
	t.Helper()
	dir := paths.FontsDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, data, 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

// runCmd executes a command through Update and returns the new model.
func runCmd(t *testing.T, m Model, cmd tea.Cmd) Model {
	t.Helper()
	if cmd == nil {
		t.Fatal("expected a command, got nil")
	}
	return updateModel(t, m, cmd())
}

// pressEnter presses enter and runs the resulting command (the import
// flow always returns importCmd from its enter key).
func pressEnter(t *testing.T, m Model) Model {
	t.Helper()
	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	return runCmd(t, next.(Model), cmd)
}

// pressKey sends a rune key and runs the command it produced (the clash
// prompt's resolution keys return importCmd).
func pressKey(t *testing.T, m Model, s string) Model {
	t.Helper()
	next, cmd := m.Update(keyRunes(s))
	return runCmd(t, next.(Model), cmd)
}

// openImport types the import overlay path without submitting it.
func openImport(t *testing.T, m Model, path string) Model {
	t.Helper()
	m = updateModel(t, m, keyRunes("i"))
	if m.overlay != overlayImport {
		t.Fatalf("i did not open the import overlay (overlay=%v)", m.overlay)
	}
	return updateModel(t, m, keyRunes(path))
}

func TestImport_Flow(t *testing.T) {
	useTermuxHome(t)
	noReload(t)

	m := NewModel()
	src := stageFont(t, "newfont.ttf")
	m = openImport(t, m, src)
	m = pressEnter(t, m)
	if !strings.HasPrefix(m.status, "Imported ") {
		t.Fatalf("status = %q, want it to start with %q", m.status, "Imported ")
	}
	if m.overlay != overlayNone {
		t.Fatalf("overlay = %v after import, want overlayNone", m.overlay)
	}
	if got := len(m.VisibleEntries()); got != 1 {
		t.Fatalf("visible entries = %d, want 1 (library gained the font)", got)
	}
	if _, err := os.Stat(filepath.Join(paths.FontsDir(), "newfont.ttf")); err != nil {
		t.Fatalf("library missing the imported font: %v", err)
	}
}

func TestImport_ClashPromptKeepBoth(t *testing.T) {
	useTermuxHome(t)
	noReload(t)
	original := fixtureBytes(t, "b.ttf")
	writeLibraryFont(t, "newfont.ttf", original)
	src := stageFont(t, "newfont.ttf")

	m := NewModel()
	m = openImport(t, m, src)
	m = pressEnter(t, m)
	if m.overlay != overlayImportClash {
		t.Fatalf("overlay = %v after a clashing import, want overlayImportClash", m.overlay)
	}
	if !strings.Contains(m.status, "File exists") {
		t.Fatalf("status = %q, want it to announce the clash", m.status)
	}
	if m.pendingImportPath != src {
		t.Fatalf("pendingImportPath = %q, want the source path %q", m.pendingImportPath, src)
	}
	view := m.View()
	for _, want := range []string{"File already exists", "keep both", "replace"} {
		if !strings.Contains(view, want) {
			t.Fatalf("clash prompt missing %q:\n%s", want, view)
		}
	}

	m = pressKey(t, m, "1")
	if !strings.HasPrefix(m.status, "Imported ") {
		t.Fatalf("status after keep-both = %q, want it to start with %q", m.status, "Imported ")
	}
	if m.overlay != overlayNone {
		t.Fatalf("overlay = %v after resolution, want overlayNone", m.overlay)
	}
	kept, err := os.ReadFile(filepath.Join(paths.FontsDir(), "newfont-1.ttf"))
	if err != nil {
		t.Fatalf("keep-both did not create newfont-1.ttf: %v", err)
	}
	if string(kept) != string(fixtureBytes(t, "a.ttf")) {
		t.Fatal("kept file content differs from the imported fixture")
	}
	still, err := os.ReadFile(filepath.Join(paths.FontsDir(), "newfont.ttf"))
	if err != nil || string(still) != string(original) {
		t.Fatalf("pre-existing library file changed (err=%v)", err)
	}
}

func TestImport_ClashPromptLetterIKeepsBoth(t *testing.T) {
	useTermuxHome(t)
	noReload(t)
	original := fixtureBytes(t, "b.ttf")
	writeLibraryFont(t, "newfont.ttf", original)
	src := stageFont(t, "newfont.ttf")

	m := NewModel()
	m = openImport(t, m, src)
	m = pressEnter(t, m)
	m = pressKey(t, m, "i") // alias for 1 (keep both)
	if !strings.HasPrefix(m.status, "Imported ") {
		t.Fatalf("status after i = %q, want it to start with %q", m.status, "Imported ")
	}
	if m.overlay != overlayNone {
		t.Fatalf("overlay = %v after resolution, want overlayNone", m.overlay)
	}
	kept, err := os.ReadFile(filepath.Join(paths.FontsDir(), "newfont-1.ttf"))
	if err != nil {
		t.Fatalf("i did not keep both (no newfont-1.ttf): %v", err)
	}
	if string(kept) != string(fixtureBytes(t, "a.ttf")) {
		t.Fatal("i kept the wrong bytes")
	}
	if got, _ := os.ReadFile(filepath.Join(paths.FontsDir(), "newfont.ttf")); string(got) != string(original) {
		t.Fatal("i must leave the pre-existing file untouched")
	}
}

func TestImport_ClashPromptReplace(t *testing.T) {
	useTermuxHome(t)
	noReload(t)
	original := fixtureBytes(t, "b.ttf")
	writeLibraryFont(t, "newfont.ttf", original)
	src := stageFont(t, "newfont.ttf")

	m := NewModel()
	m = openImport(t, m, src)
	m = pressEnter(t, m)
	if m.overlay != overlayImportClash {
		t.Fatalf("overlay = %v after a clashing import, want overlayImportClash", m.overlay)
	}

	m = pressKey(t, m, "2")
	if !strings.HasPrefix(m.status, "Imported ") {
		t.Fatalf("status after replace = %q, want it to start with %q", m.status, "Imported ")
	}
	got, err := os.ReadFile(filepath.Join(paths.FontsDir(), "newfont.ttf"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(fixtureBytes(t, "a.ttf")) {
		t.Fatal("library file was not replaced with the imported fixture")
	}
	if leftovers, _ := filepath.Glob(filepath.Join(paths.FontsDir(), "newfont-1.ttf")); len(leftovers) != 0 {
		t.Fatalf("replace must not create a keep-both sibling, found %v", leftovers)
	}
}

func TestImport_ClashPromptLetterKeys(t *testing.T) {
	useTermuxHome(t)
	noReload(t)
	writeLibraryFont(t, "newfont.ttf", fixtureBytes(t, "b.ttf"))
	src := stageFont(t, "newfont.ttf")

	m := NewModel()
	m = openImport(t, m, src)
	m = pressEnter(t, m)
	m = pressKey(t, m, "r") // alias for 2 (replace)
	if !strings.HasPrefix(m.status, "Imported ") {
		t.Fatalf("status after r = %q, want it to start with %q", m.status, "Imported ")
	}
	got, err := os.ReadFile(filepath.Join(paths.FontsDir(), "newfont.ttf"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(fixtureBytes(t, "a.ttf")) {
		t.Fatal("r did not resolve the clash by replacement")
	}
}

func TestImport_ClashPromptCancel(t *testing.T) {
	useTermuxHome(t)
	noReload(t)
	original := fixtureBytes(t, "b.ttf")
	writeLibraryFont(t, "newfont.ttf", original)
	src := stageFont(t, "newfont.ttf")

	m := NewModel()
	m = openImport(t, m, src)
	m = pressEnter(t, m)
	if m.overlay != overlayImportClash {
		t.Fatalf("overlay = %v, want overlayImportClash", m.overlay)
	}

	m = updateModel(t, m, tea.KeyMsg{Type: tea.KeyEsc})
	if m.status != "Import cancelled" {
		t.Fatalf("status = %q, want %q", m.status, "Import cancelled")
	}
	if m.overlay != overlayNone {
		t.Fatalf("overlay = %v after cancel, want overlayNone", m.overlay)
	}
	fonts, _ := filepath.Glob(filepath.Join(paths.FontsDir(), "*.ttf"))
	if len(fonts) != 1 || filepath.Base(fonts[0]) != "newfont.ttf" {
		t.Fatalf("library after cancel = %v, want only the pre-existing newfont.ttf", fonts)
	}
	still, err := os.ReadFile(filepath.Join(paths.FontsDir(), "newfont.ttf"))
	if err != nil || string(still) != string(original) {
		t.Fatalf("pre-existing library file changed on cancel (err=%v)", err)
	}
}

func TestImport_MissingPath(t *testing.T) {
	useTermuxHome(t)
	noReload(t)

	m := NewModel()
	missing := filepath.Join(t.TempDir(), "missing.ttf")
	m = openImport(t, m, missing)
	m = pressEnter(t, m)
	if !strings.HasPrefix(m.status, "Import failed") {
		t.Fatalf("status = %q, want it to start with %q", m.status, "Import failed")
	}
	if m.overlay != overlayNone {
		t.Fatalf("overlay = %v after a failed import, want overlayNone", m.overlay)
	}
}

func TestImport_HomeExpansion(t *testing.T) {
	root := useTermuxHome(t)
	noReload(t)
	home := t.TempDir()
	t.Setenv("HOME", home)
	if err := os.WriteFile(filepath.Join(home, "import.ttf"), fixtureBytes(t, "a.ttf"), 0o644); err != nil {
		t.Fatal(err)
	}

	m := NewModel()
	m = openImport(t, m, "~/import.ttf")
	m = pressEnter(t, m)
	if !strings.HasPrefix(m.status, "Imported ") {
		t.Fatalf("status = %q, want it to start with %q", m.status, "Imported ")
	}
	if _, err := os.Stat(filepath.Join(paths.FontsDir(), "import.ttf")); err != nil {
		t.Fatalf("font did not land in the TERMUX_HOME library: %v (root=%s)", err, root)
	}
}

func TestImport_TabCompletesPath(t *testing.T) {
	useTermuxHome(t)
	noReload(t)
	dir := t.TempDir()
	src := filepath.Join(dir, "newfont.ttf")
	if err := os.WriteFile(src, fixtureBytes(t, "a.ttf"), 0o644); err != nil {
		t.Fatal(err)
	}

	m := NewModel()
	m = openImport(t, m, filepath.Join(dir, "newfo"))
	if got, want := m.importInput.Value(), filepath.Join(dir, "newfo"); got != want {
		t.Fatalf("typed input = %q, want %q", got, want)
	}
	m = updateModel(t, m, tea.KeyMsg{Type: tea.KeyTab})
	if got := m.importInput.Value(); got != src {
		t.Fatalf("input after tab = %q, want it completed to %q", got, src)
	}
	if m.overlay != overlayImport {
		t.Fatalf("tab left overlay %v, want overlayImport", m.overlay)
	}
}

func TestQuit_CtrlCRestoresAndQuits(t *testing.T) {
	useTermuxHome(t)
	noReload(t)
	seedLibrary(t, "Hack.ttf")

	m := NewModel()
	m = updateModel(t, m, keyRunes(" ")) // preview → dirty
	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	m = next.(Model)
	if cmd == nil {
		t.Fatal("ctrl+c returned no cmd")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatalf("ctrl+c cmd = %T, want tea.QuitMsg", cmd())
	}
	if m.Dirty() {
		t.Fatal("ctrl+c quit left an uncommitted preview")
	}
}

func TestSuspend_CtrlZ(t *testing.T) {
	useTermuxHome(t)
	noReload(t)
	seedLibrary(t, "Hack.ttf")

	m := NewModel()
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlZ})
	if cmd == nil {
		t.Fatal("ctrl+z returned no cmd")
	}
	if _, ok := cmd().(tea.SuspendMsg); !ok {
		t.Fatalf("ctrl+z cmd = %T, want tea.SuspendMsg", cmd())
	}
}

func TestResume_RescansLibrary(t *testing.T) {
	useTermuxHome(t)
	noReload(t)

	m := NewModel()
	if got := len(m.VisibleEntries()); got != 0 {
		t.Fatalf("empty library visible entries = %d, want 0", got)
	}
	seedLibrary(t, "Hack.ttf") // an external change while suspended
	m = updateModel(t, m, tea.ResumeMsg{})
	if got := len(m.VisibleEntries()); got != 1 {
		t.Fatalf("resume did not re-read the library: got %d entries, want 1", got)
	}
}

// TestPreviewPane_CachesDetails pins the no-disk-I/O-in-View contract:
// once the metadata is cached, a later render must not re-read the file.
func TestPreviewPane_CachesDetails(t *testing.T) {
	useTermuxHome(t)
	noReload(t)
	seedLibrary(t, "Hack.ttf")

	m := NewModel()
	if !strings.Contains(m.PreviewPane(), "glyphs") {
		t.Fatal("precondition: detail line missing from the preview")
	}
	e, ok := m.selectedEntry()
	if !ok {
		t.Fatal("no selected entry")
	}
	// Corrupt the file on disk; a cached pane must not re-read it.
	if err := os.WriteFile(e.Path, []byte("not a font"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(m.PreviewPane(), "glyphs") {
		t.Fatal("PreviewPane re-read the font on render instead of using the cache")
	}
}

func TestSyncDetail_TracksSelection(t *testing.T) {
	useTermuxHome(t)
	noReload(t)
	seedLibrary(t, "Alpha.ttf", "Beta.ttf")

	m := NewModel()
	first := m.detailPath
	if first == "" {
		t.Fatal("no detail cached for the initial selection")
	}
	m = updateModel(t, m, tea.KeyMsg{Type: tea.KeyDown})
	if m.detailPath == first {
		t.Fatalf("detailPath still %q after moving the cursor", m.detailPath)
	}
	want, _ := m.selectedEntry()
	if m.detailPath != want.Path {
		t.Fatalf("detailPath = %q, want the selected entry %q", m.detailPath, want.Path)
	}
}

// TestLayout_NoLineExceedsWidth pins the width-aware footer and layout:
// no rendered line may be wider than the terminal (which would wrap).
func TestLayout_NoLineExceedsWidth(t *testing.T) {
	useTermuxHome(t)
	noReload(t)
	seedLibrary(t, "Alpha.ttf", "Beta.ttf")

	for _, size := range [][2]int{{80, 24}, {100, 40}, {60, 20}, {44, 8}} {
		m := NewModel()
		m.width, m.height = size[0], size[1]
		m.sizeWidgets()
		for _, line := range strings.Split(m.View(), "\n") {
			if w := lipgloss.Width(line); w > size[0] {
				t.Fatalf("%dx%d: line is %d cells wide: %q", size[0], size[1], w, line)
			}
		}
	}
}

func TestView_TooSmallNotice(t *testing.T) {
	useTermuxHome(t)
	noReload(t)
	seedLibrary(t, "Hack.ttf")

	m := NewModel()
	m.width, m.height = 30, 24
	m.sizeWidgets()
	if !m.tooSmall {
		t.Fatal("30x24 should be flagged too small")
	}
	view := m.View()
	if !strings.Contains(view, "too small") {
		t.Fatalf("no too-small notice:\n%s", view)
	}
	if strings.Contains(view, "Hack.ttf") {
		t.Fatalf("too-small view still renders the library:\n%s", view)
	}
}

func TestView_ShortFoldsPreview(t *testing.T) {
	useTermuxHome(t)
	noReload(t)
	seedLibrary(t, "Alpha.ttf", "Beta.ttf")

	m := NewModel()
	m.width, m.height = 60, 12
	m.sizeWidgets()
	if !m.folded {
		t.Fatal("a 12-row terminal should fold the preview away")
	}
	view := m.View()
	if hasSideBySideBoxes(view) {
		t.Fatalf("folded layout shows two columns:\n%s", view)
	}
	if !strings.Contains(view, "Alpha.ttf") {
		t.Fatalf("folded layout dropped the library:\n%s", view)
	}
	if strings.Contains(view, "AaBbCc") {
		t.Fatalf("folded layout still renders the preview pane:\n%s", view)
	}
}

// A narrow but tall terminal keeps the vertical stack: width no longer
// forces the preview away, only height does.
func TestView_VerticalWorksNarrow(t *testing.T) {
	useTermuxHome(t)
	noReload(t)
	seedLibrary(t, "Alpha.ttf", "Beta.ttf")

	m := NewModel()
	m.width, m.height = 60, 24
	m.sizeWidgets()
	if m.folded {
		t.Fatal("a 60x24 terminal should keep the preview")
	}
	view := m.View()
	if hasSideBySideBoxes(view) {
		t.Fatalf("narrow layout should stack, not sit side by side:\n%s", view)
	}
	if !strings.Contains(view, "╭─ Preview") || !strings.Contains(view, "╭─ Library") {
		t.Fatalf("narrow stack missing a pane:\n%s", view)
	}
	if !strings.Contains(view, "AaBbCc") {
		t.Fatalf("narrow stack dropped the preview content:\n%s", view)
	}
}

func TestLayout_NarrowDownloadFits(t *testing.T) {
	useTermuxHome(t)
	noReload(t)
	seedLibrary(t, "Alpha.ttf", "Beta.ttf")

	m := NewModel()
	m.width, m.height = 60, 20
	m.sizeWidgets()
	m = updateModel(t, m, keyRunes("d"))
	view := m.View()
	if h := lipgloss.Height(view); h != 20 {
		t.Fatalf("narrow download height = %d, want 20", h)
	}
	for _, line := range strings.Split(view, "\n") {
		if w := lipgloss.Width(line); w > 60 {
			t.Fatalf("narrow download line is %d cells wide: %q", w, line)
		}
	}
	if hasSideBySideBoxes(view) {
		t.Fatalf("narrow download should stack, not sit side by side:\n%s", view)
	}
}

// TestLayout_LiveDownloadStaysInBounds pins width discipline while a fetch
// runs: neither the progress bar nor the "Downloading …" line may overrun
// the download box in either layout.
func TestLayout_LiveDownloadStaysInBounds(t *testing.T) {
	useTermuxHome(t)
	noReload(t)
	seedLibrary(t, "Alpha.ttf", "Beta.ttf")

	for _, size := range [][2]int{{100, 40}, {60, 20}} {
		m := NewModel()
		m.width, m.height = size[0], size[1]
		m.sizeWidgets()
		m = updateModel(t, m, keyRunes("d"))
		m = updateModel(t, m, downloadStartMsg{name: "Some-Extremely-Long-Nerd-Font-Family-Name"})
		m = updateModel(t, m, downloadProgressMsg{frac: 0.42, gen: m.dlGen, known: true})
		for _, line := range strings.Split(m.View(), "\n") {
			if w := lipgloss.Width(line); w > size[0] {
				t.Fatalf("%dx%d: line is %d cells wide: %q", size[0], size[1], w, line)
			}
		}
	}
}

func TestASCII_PlainChrome(t *testing.T) {
	useTermuxHome(t)
	noReload(t)
	seedLibrary(t, "Hack.ttf")
	src, err := os.ReadFile(filepath.Join(paths.FontsDir(), "Hack.ttf"))
	if err != nil {
		t.Fatal(err)
	}
	slot, err := paths.FontSlotPath("regular")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(slot, src, 0o644); err != nil {
		t.Fatal(err)
	}

	SetASCII(true)
	defer SetASCII(false)
	m := NewModel()
	m.width, m.height = 60, 24
	m.sizeWidgets()
	view := m.View()
	for _, bad := range []string{"╭", "●", "…", "▶"} {
		if strings.Contains(view, bad) {
			t.Fatalf("ASCII mode leaked %q:\n%s", bad, view)
		}
	}
	if !strings.Contains(view, "+-") {
		t.Fatalf("ASCII mode lost the plain border:\n%s", view)
	}
	if !strings.Contains(view, "* regular") {
		t.Fatalf("ASCII mode lost the applied badge:\n%s", view)
	}
}
