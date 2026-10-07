// Package tui implements the Bubble Tea font picker: browse the font
// library, filter, manual live preview (Space/p), commit (Enter), restore
// (Esc), slot cycling (s), import (i) and Nerd Font download (d).
//
// The list is focused on launch so arrows move immediately; Tab reaches the
// filter. Highlighting only updates the info pane, never applies a font.
// Downloads run as tea.Cmd values producing messages, and every I/O error
// surfaces as status-line text instead of crashing.
package tui

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"termux-fonts-go/internal/apply"
	"termux-fonts-go/internal/downloader"
	"termux-fonts-go/internal/paths"
	"termux-fonts-go/internal/scan"
	"termux-fonts-go/internal/theme"
)

// FilterMsg sets the list filter to its value and narrows visible items.
type FilterMsg string

type downloadStartMsg struct{ name string }

type downloadProgressMsg float64

type downloadDoneMsg struct {
	path string
	err  error
	gen  int
}

type importDoneMsg struct {
	path string
	err  error
}

type overlay int

const (
	overlayNone overlay = iota
	overlayImport
	overlayDownload
)

// capturePromptLines renders the user's live shell prompt (with colors)
// once per session. Anything failing — unknown shell, timeout, empty
// output — yields nil and the caller falls back to the mock prompt.
var capturePromptLines = defaultCapturePromptLines

func defaultCapturePromptLines() []string {
	shell := os.Getenv("SHELL")
	var argv []string
	switch {
	case strings.HasSuffix(shell, "zsh"):
		argv = []string{shell, "-ic", `print -P "$PROMPT"`}
	case strings.HasSuffix(shell, "bash"):
		argv = []string{shell, "-ic", `echo "${PS1@P}"`}
	default:
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	var buf bytes.Buffer
	cmd.Stdout = &buf
	if err := cmd.Run(); err != nil {
		return nil
	}
	var lines []string
	for _, ln := range strings.Split(buf.String(), "\n") {
		ln = strings.TrimRight(ln, " \t")
		if ln == "" {
			continue
		}
		if r := []rune(ln); len(r) > 200 {
			ln = string(r[:200])
		}
		lines = append(lines, ln)
		if len(lines) == 2 {
			break
		}
	}
	if len(lines) == 0 {
		return nil
	}
	return lines
}

// slotOrder is the fixed s-key cycle.
var slotOrder = []string{"regular", "bold", "italic", "bold-italic"}

// themePalette loads the Termux palette; any failure means defaults.
func themePalette() theme.Palette {
	p, err := theme.LoadFile(paths.TermuxDir() + "/colors.properties")
	if err != nil {
		return theme.Palette{}
	}
	return p
}

// listLibrary and restoreOriginal are package-level vars (rather than
// direct calls) so tests can inject load/restore failures TTY-free.
var (
	listLibrary     = scan.ListLibrary
	restoreOriginal = apply.RestoreOriginal
)

// Model is the Elm state: entries, filter, slot, session, status and
// overlay/download progress.
type Model struct {
	entries  []scan.FontEntry
	list     list.Model
	delegate fontDelegate
	filter   textinput.Model

	focusFilter bool
	slot        string
	state       *apply.SessionState
	status      string
	width       int
	height      int

	overlay     overlay
	importInput textinput.Model

	previewH int // preview content height from the weight split (0 = natural)

	prompt []string // live shell prompt lines (nil → mock fallback)

	applied map[string]string // library path → "● slot" badges

	dlNames    []string
	dlCursor   int
	dlActive   bool
	dlName     string
	dlProgress float64
	dlGen      int

	spinner  spinner.Model
	progress progress.Model
}

// NewModel loads the library and returns a list-focused model.
// A library load failure surfaces as status text, never a silent empty list.
func NewModel() Model {
	entries, loadErr := listLibrary()
	if entries == nil {
		entries = []scan.FontEntry{}
	}
	status := "space preview · enter keep · tab filter"
	if loadErr != nil {
		status = "Library load failed: " + loadErr.Error()
	}
	filter := textinput.New()
	filter.Placeholder = "Filter fonts..."
	filter.CharLimit = 64

	l := list.New(nil, fontDelegate{}, 40, 14)
	l.SetShowTitle(false)
	l.SetShowStatusBar(false)
	l.SetShowHelp(false)
	l.SetShowPagination(false)
	l.SetFilteringEnabled(false)
	l.DisableQuitKeybindings()

	importInput := textinput.New()
	importInput.Placeholder = "/sdcard/Download/Hack.ttf"
	importInput.CharLimit = 256

	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(accent)

	bar := progress.New(progress.WithSolidFill("#7C6FF0"), progress.WithWidth(40))

	names := make([]string, 0, len(downloader.NerdFonts))
	for name := range downloader.NerdFonts {
		names = append(names, name)
	}
	sort.Strings(names)

	applyTheme(themePalette())
	m := Model{
		entries:     entries,
		list:        l,
		delegate:    fontDelegate{},
		filter:      filter,
		slot:        "regular",
		state:       apply.NewSessionState(),
		status:      status,
		importInput: importInput,
		dlNames:     names,
		spinner:     sp,
		progress:    bar,
		applied:     appliedBadges(entries),
		prompt:      capturePromptLines(),
	}
	m.refreshItems()
	return m
}

// InitialModel is the entry point for tea.NewProgram.
func InitialModel() Model { return NewModel() }

// Init focuses the list (arrows move immediately) and issues no commands.
func (m Model) Init() tea.Cmd { return nil }

// VisibleEntries returns the currently unfiltered-out library entries.
func (m Model) VisibleEntries() []scan.FontEntry {
	out := []scan.FontEntry{}
	for _, item := range m.list.Items() {
		if fi, ok := item.(fontItem); ok {
			out = append(out, fi.entry)
		}
	}
	return out
}

// Dirty reports whether a preview is awaiting commit/restore.
func (m Model) Dirty() bool { return apply.IsPreviewDirty(m.state) }

// Downloading reports whether a download overlay/progress is active.
func (m Model) Downloading() bool { return m.dlActive }

// DownloadProgress returns the current download fraction in [0, 1].
func (m Model) DownloadProgress() float64 { return m.dlProgress }

func (m Model) selectedEntry() (scan.FontEntry, bool) {
	item := m.list.SelectedItem()
	fi, ok := item.(fontItem)
	if !ok {
		return scan.FontEntry{}, false
	}
	return fi.entry, true
}

// appliedBadges maps library entry paths to "● slot" badges by comparing
// file bytes with the live slot files. Size-prefiltered, so the common
// case costs 4 slot reads and no library hashing.
func appliedBadges(entries []scan.FontEntry) map[string]string {
	out := map[string]string{}
	for slot := range paths.SlotFiles {
		target, err := paths.FontSlotPath(slot)
		if err != nil {
			continue
		}
		sb, err := os.ReadFile(target)
		if err != nil {
			continue
		}
		sh := sha256.Sum256(sb)
		for _, e := range entries {
			if e.Size != int64(len(sb)) {
				continue
			}
			eb, err := os.ReadFile(e.Path)
			if err != nil {
				continue
			}
			if sha256.Sum256(eb) == sh {
				if out[e.Path] != "" {
					out[e.Path] += " "
				}
				out[e.Path] += "● " + slot
			}
		}
	}
	return out
}

func (m *Model) refreshItems() {
	q := strings.ToLower(strings.TrimSpace(m.filter.Value()))
	items := make([]list.Item, 0, len(m.entries))
	for _, e := range m.entries {
		if q == "" || strings.Contains(strings.ToLower(e.Name), q) ||
			strings.Contains(strings.ToLower(e.Family), q) {
			items = append(items, fontItem{entry: e, badge: m.applied[e.Path]})
		}
	}
	m.list.SetItems(items)
	m.delegate.query = strings.TrimSpace(m.filter.Value())
	m.list.SetDelegate(m.delegate)
}

// Update routes key, filter and download messages; cursor movement only
// changes the highlight and never touches slot files.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.sizeWidgets()
		return m, nil
	case FilterMsg:
		m.filter.SetValue(string(msg))
		m.refreshItems()
		return m, nil
	case downloadStartMsg:
		m.dlGen++
		m.dlActive = true
		m.dlName = msg.name
		m.dlProgress = 0
		// Fetch reports no byte fractions, so the bar is an activity
		// indicator: say so instead of implying measured progress.
		m.status = fmt.Sprintf("Downloading %s… (no progress info)", msg.name)
		return m, tea.Batch(fetchCmd(msg.name, m.dlGen), m.spinner.Tick)
	case downloadProgressMsg:
		m.dlProgress = clamp01(float64(msg))
		return m, nil
	case downloadDoneMsg:
		m.dlActive = false
		m.dlProgress = 0
		if msg.gen != m.dlGen {
			// Orphaned by a dismiss: refresh the list quietly without
			// claiming a download the user was told was dismissed.
			m.rescan()
			return m, nil
		}
		m.overlay = overlayNone
		if msg.err != nil {
			m.status = "Download failed: " + msg.err.Error()
		} else {
			m.status = "Downloaded " + baseName(msg.path)
			m.rescan()
		}
		return m, nil
	case importDoneMsg:
		m.overlay = overlayNone
		m.importInput.Blur()
		if msg.err != nil {
			m.status = "Import failed: " + msg.err.Error()
		} else {
			m.status = "Imported " + baseName(msg.path)
			m.rescan()
		}
		return m, nil
	case spinner.TickMsg:
		if m.dlActive {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			// Ease the bar toward 90% while the fetch runs; the done
			// message dismisses it. Real progress arrives via
			// downloadProgressMsg when available.
			if m.dlProgress < 0.9 {
				m.dlProgress += (0.9 - m.dlProgress) * 0.08
			}
			return m, cmd
		}
		return m, nil
	case tea.KeyMsg:
		return m.handleKey(msg)
	}
	if m.focusFilter && m.overlay == overlayNone {
		var cmd tea.Cmd
		m.filter, cmd = m.filter.Update(msg)
		m.refreshItems()
		return m, cmd
	}
	return m, nil
}

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	if m.overlay == overlayImport {
		switch key {
		case "esc":
			m.overlay = overlayNone
			m.importInput.Blur()
			m.status = "Import cancelled"
			return m, nil
		case "enter":
			return m, importCmd(strings.TrimSpace(m.importInput.Value()))
		}
		var cmd tea.Cmd
		m.importInput, cmd = m.importInput.Update(msg)
		return m, cmd
	}

	if m.overlay == overlayDownload {
		switch key {
		case "esc", "q":
			if m.dlActive {
				// The fetch cannot be cancelled mid-flight; orphan it so
				// its late completion refreshes quietly instead of
				// reporting a download the user dismissed.
				m.dlGen++
				m.dlActive = false
				m.dlProgress = 0
				m.overlay = overlayNone
				m.status = "Download dismissed — finishing in background"
				return m, nil
			}
			m.overlay = overlayNone
			m.status = "Download cancelled"
			return m, nil
		case "enter":
			if !m.dlActive && len(m.dlNames) > 0 {
				name := m.dlNames[m.dlCursor]
				return m, func() tea.Msg { return downloadStartMsg{name: name} }
			}
			return m, nil
		case "up", "k":
			if m.dlCursor > 0 {
				m.dlCursor--
			}
			return m, nil
		case "down", "j":
			if m.dlCursor < len(m.dlNames)-1 {
				m.dlCursor++
			}
			return m, nil
		}
		return m, nil
	}

	if m.focusFilter {
		switch key {
		case "tab", "enter":
			m.focusFilter = false
			m.filter.Blur()
			return m, nil
		case "esc":
			m.focusFilter = false
			m.filter.Blur()
			m.filter.SetValue("")
			m.refreshItems()
			m.status = "Filter cleared"
			return m, nil
		}
		var cmd tea.Cmd
		m.filter, cmd = m.filter.Update(msg)
		m.refreshItems()
		return m, cmd
	}

	switch key {
	case "q":
		if apply.IsPreviewDirty(m.state) {
			if _, err := restoreOriginal(m.state); err != nil {
				m.status = "Restore failed: " + err.Error()
				return m, nil
			}
			m.status = "Restored original — bye"
		}
		return m, tea.Quit
	case "esc":
		if apply.IsPreviewDirty(m.state) {
			if _, err := restoreOriginal(m.state); err != nil {
				m.status = "Restore failed: " + err.Error()
			} else {
				m.status = "Restored original"
			}
		} else {
			m.status = "Nothing to restore"
		}
		return m, nil
	case "tab":
		m.focusFilter = true
		m.filter.Focus()
		return m, textinput.Blink
	case "s":
		m.cycleSlot()
		return m, nil
	case " ", "p":
		m.doPreview()
		return m, nil
	case "enter":
		m.doCommit()
		return m, nil
	case "i":
		m.overlay = overlayImport
		m.importInput.Focus()
		m.status = "Import: type a font path — Enter imports, Esc cancels"
		return m, textinput.Blink
	case "d":
		m.overlay = overlayDownload
		m.dlCursor = 0
		m.status = "Download: ↑/↓ choose — Enter downloads, Esc cancels"
		return m, nil
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m *Model) sizeWidgets() {
	// Weight split of the space below title/filter and above
	// status/help: preview 2 : list 3. Borders add naturally on top
	// of these content heights (never set Height on bordered styles).
	listW, listH := 88, 14
	if m.width > 0 {
		listW = max(m.width-8, 30)
	}
	m.previewH = 0
	if m.height > 0 {
		avail := max(m.height-4, 4)
		previewOuter := max(avail*2/5, 3)
		listOuter := max(avail-previewOuter, 2)
		m.previewH = max(previewOuter-2, 1)
		listH = max(listOuter-2, 1)
	}
	m.list.SetSize(listW, listH)
	m.delegate.width = listW
	m.list.SetDelegate(m.delegate)
	m.progress.Width = max(listW, 20)
}
