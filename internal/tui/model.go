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

	"github.com/GeneralKaos666/nerdfont-changer/internal/apply"
	"github.com/GeneralKaos666/nerdfont-changer/internal/downloader"
	"github.com/GeneralKaos666/nerdfont-changer/internal/paths"
	"github.com/GeneralKaos666/nerdfont-changer/internal/scan"
	"github.com/GeneralKaos666/nerdfont-changer/internal/theme"
)

// FilterMsg sets the list filter to its value and narrows visible items.
type FilterMsg string

// promptMsg carries the shell-prompt lines captured by the Init command,
// so the (up to 3s) capture never blocks the first frame.
type promptMsg []string

type overlay int

const (
	overlayNone overlay = iota
	overlayImport
	overlayDownload
	overlayImportClash
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

// themePalette loads the Termux palette; any failure means defaults.
func themePalette() theme.Palette {
	p, err := theme.LoadFile(paths.ColorsPath())
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

	overlay           overlay
	importInput       textinput.Model
	pendingImportPath string // source path of an import awaiting the clash choice

	previewH int  // preview content height (0 = natural)
	previewW int  // preview box content width
	leftW    int  // left column box content width
	rightW   int  // right column box content width
	contentW int  // full-width content width in single-pane (narrow) mode
	listRows int  // library list rows inside its box
	band     int  // column band height (box outer height)
	narrow   bool // terminal narrower than wideMin: preview folds away
	tooSmall bool // terminal below the hard minimum: View shows a notice

	// Cached font metadata for the selected entry. View never touches the
	// disk; syncDetail refreshes this only when the selection changes.
	detail     scan.FontDetails
	detailPath string // path the cached detail describes ("" = nothing selected)
	hasDetail  bool   // detail parsed successfully for detailPath

	prompt []string // live shell prompt lines (nil → mock fallback)

	applied map[string]string // library path → "● slot" badges

	dlNames         []string
	dlFiltered      []string
	dlFilter        textinput.Model
	dlFilterFocused bool
	dlCursor        int
	dlOffset        int
	dlActive        bool
	dlName          string
	dlProgress      float64
	dlGen           int
	dlKnown         bool         // a measured total arrived; wording and easing stop faking it
	dlCh            chan tea.Msg // in-flight fetch pipeline; nil when none is current

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
	status := "Ready"
	if loadErr != nil {
		status = "Library load failed: " + loadErr.Error()
	}
	filter := textinput.New()
	filter.Placeholder = "Filter fonts (tab)"
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

	dlFilter := textinput.New()
	dlFilter.Placeholder = "Type to filter Nerd Fonts..."
	dlFilter.CharLimit = 64

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
		dlFiltered:  names,
		dlFilter:    dlFilter,
		spinner:     sp,
		progress:    bar,
		applied:     appliedBadges(entries),
	}
	m.refreshItems()
	return m
}

// InitialModel is the entry point for tea.NewProgram.
func InitialModel() Model { return NewModel() }

// Init starts the model: the list is already focused (arrows move
// immediately), and the shell-prompt capture runs as a command so the
// first frame is never held up by it.
func (m Model) Init() tea.Cmd {
	return func() tea.Msg { return promptMsg(capturePromptLines()) }
}

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

// syncDetail refreshes the cached font metadata for the selected entry.
// It runs from Update (on cursor/filter/library changes), never from
// View, so rendering stays free of disk I/O even mid-download.
func (m *Model) syncDetail() {
	e, ok := m.selectedEntry()
	if !ok {
		m.detail = scan.FontDetails{}
		m.detailPath = ""
		m.hasDetail = false
		return
	}
	if e.Path == m.detailPath {
		return // cache hit: same file, same tables
	}
	m.detailPath = e.Path
	d, err := scan.Describe(e.Path)
	m.detail, m.hasDetail = d, err == nil
}

// appliedBadges maps library entry paths to "<marker> slot" badges by
// comparing file bytes with the live slot files. Size-prefiltered, so the
// common case costs 4 slot reads and no library hashing.
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
				out[e.Path] += badgeDot() + " " + slot
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
	m.syncDetail()
}
