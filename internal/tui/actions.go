package tui

import (
	"fmt"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"termux-fonts-go/internal/apply"
	"termux-fonts-go/internal/downloader"
	"termux-fonts-go/internal/importer"
	"termux-fonts-go/internal/scan"
)

func baseName(p string) string { return filepath.Base(p) }

func (m *Model) rescan() {
	entries, err := scan.ListLibrary()
	if err != nil {
		m.status = "Rescan failed: " + err.Error()
		return
	}
	m.entries = entries
	m.refreshItems()
}

func (m *Model) cycleSlot() {
	for i, s := range slotOrder {
		if s == m.slot {
			m.slot = slotOrder[(i+1)%len(slotOrder)]
			break
		}
	}
	m.status = "Slot: " + m.slot
}

func reloadHint() string {
	if ok := apply.LastReloadOK(); ok != nil && !*ok {
		return " (" + apply.ManualRestartHint + ")"
	}
	return ""
}

func (m *Model) doPreview() {
	e, ok := m.selectedEntry()
	if !ok {
		m.status = "No font selected"
		return
	}
	if _, err := apply.PreviewFont(e.Path, m.slot, m.state); err != nil {
		m.status = "Preview failed: " + err.Error()
		return
	}
	m.status = fmt.Sprintf("Preview %s — Enter keeps, Esc restores", e.Name) + reloadHint()
}

func (m *Model) doCommit() {
	if apply.IsPreviewDirty(m.state) {
		target := apply.CommitPreview(m.state)
		if target == "" {
			m.status = "Nothing to commit"
			return
		}
		m.status = fmt.Sprintf("Kept %s → %s slot", baseName(target), m.slot) + reloadHint()
		return
	}
	e, ok := m.selectedEntry()
	if !ok {
		m.status = "No font selected"
		return
	}
	target, err := apply.InstallFont(e.Path, m.slot)
	if err != nil {
		m.status = "Install failed: " + err.Error()
		return
	}
	m.status = fmt.Sprintf("Installed %s → %s", baseName(target), m.slot) + reloadHint()
}

// fetchCmd downloads a Nerd Font off the Elm loop; completion (or any
// error) returns as a downloadDoneMsg, never a crash.
func fetchCmd(name string) tea.Cmd {
	return func() tea.Msg {
		dest, err := downloader.Fetch(name, false)
		return downloadDoneMsg{path: dest, err: err}
	}
}

// importCmd copies an external font into the library as a message.
func importCmd(path string) tea.Cmd {
	return func() tea.Msg {
		if strings.TrimSpace(path) == "" {
			return importDoneMsg{err: fmt.Errorf("type a font file path first")}
		}
		dest, err := importer.ImportFile(path, "keep-both")
		return importDoneMsg{path: dest, err: err}
	}
}

func clamp01(f float64) float64 {
	if f < 0 {
		return 0
	}
	if f > 1 {
		return 1
	}
	return f
}
