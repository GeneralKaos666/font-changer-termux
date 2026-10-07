package tui

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"termux-fonts-go/internal/apply"
	"termux-fonts-go/internal/paths"
	"termux-fonts-go/internal/scan"
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

	m = updateModel(t, m, downloadProgressMsg(0.5))
	if got := m.DownloadProgress(); got != 0.5 {
		t.Fatalf("download progress = %v, want 0.5", got)
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

func TestQuit_RestoreFailureStaysOpen(t *testing.T) {
	useTermuxHome(t)
	noReload(t)
	seedLibrary(t, "Hack.ttf", "JetBrainsMono.ttf")

	m := NewModel()
	m = updateModel(t, m, keyRunes(" "))
	if !m.Dirty() {
		t.Fatalf("space did not preview; status=%q", m.status)
	}

	old := restoreOriginal
	restoreOriginal = func(*apply.SessionState) (bool, error) {
		return false, errors.New("boom")
	}
	defer func() { restoreOriginal = old }()

	next, cmd := m.Update(keyRunes("q"))
	m = next.(Model)
	if cmd != nil {
		t.Fatal("q with failed restore quit instead of staying open")
	}
	if !strings.Contains(m.status, "Restore failed") {
		t.Fatalf("status = %q, want restore failure text", m.status)
	}
	if !m.Dirty() {
		t.Fatal("failed restore cleared the dirty flag")
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

func TestView_PreviewAboveList(t *testing.T) {
	useTermuxHome(t)
	noReload(t)
	seedLibrary(t, "Alpha.ttf", "Beta.ttf")

	m := NewModel()
	m.width, m.height = 100, 40
	m.sizeWidgets()
	view := m.View()
	entries := m.VisibleEntries()
	if len(entries) < 2 {
		t.Fatalf("want >= 2 visible entries, got %d", len(entries))
	}
	// entries[0] is highlighted AND shown in the preview pane; entries[1]
	// appears only in the list. Order must be: preview, filter counts, list.
	sample := strings.Index(view, "AaBbCc")
	counts := strings.Index(view, "2/2")
	second := strings.Index(view, entries[1].Name)
	if sample < 0 || counts < 0 || second < 0 {
		t.Fatalf("missing section (sample=%d counts=%d second=%d)", sample, counts, second)
	}
	if !(sample < counts && counts < second) {
		t.Fatalf("wrong vertical order (sample=%d counts=%d second=%d)", sample, counts, second)
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

func TestPrompt_InjectedLinesAppear(t *testing.T) {
	useTermuxHome(t)
	noReload(t)
	seedLibrary(t, "Hack.ttf")
	old := capturePromptLines
	capturePromptLines = func() []string { return []string{"\x1b[32m╭─ injected", "╰─❯ test"} }
	defer func() { capturePromptLines = old }()

	m := NewModel()
	if pane := m.PreviewPane(); !strings.Contains(pane, "injected") {
		t.Fatalf("injected prompt missing:\n%s", pane)
	}
}

// Keep NewModel hermetic: no real shell capture in tests (per-test
// overrides still work by reassigning capturePromptLines).
func TestMain(m *testing.M) {
	capturePromptLines = func() []string { return nil }
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
