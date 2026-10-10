package tui

import (
	"fmt"
	"path/filepath"

	"termux-fonts-go/internal/apply"
	"termux-fonts-go/internal/paths"
	"termux-fonts-go/internal/scan"
)

func (m *Model) rescan() {
	entries, err := scan.ListLibrary()
	if err != nil {
		m.status = "Rescan failed: " + err.Error()
		return
	}
	m.entries = entries
	m.applied = appliedBadges(entries)
	m.refreshItems()
}

func (m *Model) cycleSlot() {
	for i, s := range paths.SlotNames {
		if s == m.slot {
			m.slot = paths.SlotNames[(i+1)%len(paths.SlotNames)]
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
		// The toast must name the committed (previewed) font and slot;
		// CommitPreview clears Preview, so read both before the call.
		src, slot := m.state.Preview.Src, m.state.Preview.Slot
		target := apply.CommitPreview(m.state)
		if target == "" {
			m.status = "Nothing to commit"
			return
		}
		m.status = fmt.Sprintf("Kept %s → %s slot", filepath.Base(src), slot) + reloadHint()
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
	m.status = fmt.Sprintf("Installed %s → %s", filepath.Base(target), m.slot) + reloadHint()
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
