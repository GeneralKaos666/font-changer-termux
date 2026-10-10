package tui

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/GeneralKaos666/font-changer-termux/internal/apply"
	"github.com/GeneralKaos666/font-changer-termux/internal/importer"
)

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
		m.dlKnown = false
		// No byte fractions yet: the bar is an activity indicator until
		// the first downloadProgressMsg with a known total arrives.
		m.status = fmt.Sprintf("Downloading %s… (no progress info)", msg.name)
		ch, cmd := startFetch(msg.name, m.dlGen)
		m.dlCh = ch
		return m, tea.Batch(cmd, m.spinner.Tick)
	case downloadProgressMsg:
		if msg.gen != m.dlGen {
			return m, nil // stale: from a dismissed or superseded fetch
		}
		m.dlKnown = msg.known
		if msg.known {
			m.dlProgress = clamp01(msg.frac)
			// Measured now — drop the indeterminate wording.
			m.status = fmt.Sprintf("Downloading %s…", m.dlName)
		}
		// else: unknown total — leave dlProgress to the spinner easing.
		if m.dlCh == nil {
			return m, nil
		}
		return m, func() tea.Msg { return <-m.dlCh } // keep polling the pipeline
	case downloadDoneMsg:
		m.dlActive = false
		m.dlProgress = 0
		m.dlKnown = false
		m.dlCh = nil // pipeline over; no further re-arming (covers both branches)
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
			m.status = "Downloaded " + filepath.Base(msg.path)
			m.rescan()
		}
		m.sizeWidgets()
		return m, nil
	case importDoneMsg:
		m.pendingImportPath = ""
		m.overlay = overlayNone
		m.importInput.Blur()
		var ce *importer.ClashError
		if errors.As(msg.err, &ce) {
			// Name collision under "ask": switch to the three-choice
			// prompt instead of reporting failure. msg.path carries the
			// source path so the retry can re-import it.
			m.overlay = overlayImportClash
			m.pendingImportPath = msg.path
			m.status = "File exists — 1 keep both, 2 replace, esc cancel"
			return m, nil
		}
		if msg.err != nil {
			m.status = "Import failed: " + msg.err.Error()
		} else {
			m.status = "Imported " + filepath.Base(msg.path)
			m.rescan()
		}
		m.sizeWidgets()
		return m, nil
	case spinner.TickMsg:
		if m.dlActive {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			// Ease the bar toward 90% only while progress is unmeasured;
			// a known fraction is owned by downloadProgressMsg. The done
			// message dismisses the bar.
			if !m.dlKnown && m.dlProgress < 0.9 {
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

	if m.overlay == overlayImportClash {
		switch key {
		case "1", "i":
			return m, importCmd(m.pendingImportPath, "keep-both")
		case "2", "r":
			return m, importCmd(m.pendingImportPath, "replace")
		case "esc":
			m.overlay = overlayNone
			m.status = "Import cancelled"
			return m, nil
		}
		return m, nil // any other key is ignored while the prompt is up
	}

	if m.overlay == overlayImport {
		switch key {
		case "esc":
			m.overlay = overlayNone
			m.importInput.Blur()
			m.status = "Import cancelled"
			return m, nil
		case "enter":
			path := expandHome(strings.TrimSpace(m.importInput.Value()))
			return m, importCmd(path, "ask")
		case "tab":
			m.importInput.SetValue(completePath(m.importInput.Value()))
			return m, nil
		}
		var cmd tea.Cmd
		m.importInput, cmd = m.importInput.Update(msg)
		return m, cmd
	}

	if m.overlay == overlayDownload {
		switch key {
		case "esc":
			if m.dlFilterFocused {
				m.dlFilterFocused = false
				m.dlFilter.Blur()
				return m, nil
			}
			if strings.TrimSpace(m.dlFilter.Value()) != "" {
				m.dlFilter.SetValue("")
				m.refreshDownloadItems()
				m.status = "Download filter cleared"
				return m, nil
			}
			return m.dismissDownload()
		case "q":
			if m.dlFilterFocused {
				var cmd tea.Cmd
				m.dlFilter, cmd = m.dlFilter.Update(msg)
				m.refreshDownloadItems()
				return m, cmd
			}
			return m.dismissDownload()
		case "tab":
			m.dlFilterFocused = !m.dlFilterFocused
			if m.dlFilterFocused {
				m.dlFilter.Focus()
				return m, textinput.Blink
			}
			m.dlFilter.Blur()
			return m, nil
		case "enter":
			if !m.dlFilterFocused && !m.dlActive && len(m.dlFiltered) > 0 {
				name := m.dlFiltered[m.dlCursor]
				return m, func() tea.Msg { return downloadStartMsg{name: name} }
			}
			return m, nil
		}
		// Arrow keys browse the list even while the filter has focus;
		// every other key edits the filter.
		switch key {
		case "up", "k":
			if !m.dlFilterFocused && m.dlCursor > 0 {
				m.dlCursor--
				m.clampDownloadWindow()
			}
			return m, nil
		case "down", "j":
			if !m.dlFilterFocused && m.dlCursor < len(m.dlFiltered)-1 {
				m.dlCursor++
				m.clampDownloadWindow()
			}
			return m, nil
		}
		if m.dlFilterFocused {
			var cmd tea.Cmd
			m.dlFilter, cmd = m.dlFilter.Update(msg)
			m.refreshDownloadItems()
			return m, cmd
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
				m.status = "Restore failed: " + err.Error() + " — quitting anyway"
			} else {
				m.status = "Restored original — bye"
			}
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
		m.dlOffset = 0
		m.dlFilterFocused = false
		m.dlFilter.SetValue("")
		m.dlFilter.Blur()
		m.refreshDownloadItems()
		m.sizeWidgets()
		m.status = "Download: ↑/↓ choose · tab filter · Enter downloads, Esc cancels"
		return m, nil
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}
