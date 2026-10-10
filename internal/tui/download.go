package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/GeneralKaos666/font-changer-termux/internal/downloader"
)

type downloadStartMsg struct{ name string }

type downloadProgressMsg struct {
	frac  float64
	gen   int
	known bool
}

type downloadDoneMsg struct {
	path string
	err  error
	gen  int
}

// dlWindow is how many Nerd Font names the download box shows at once.
const dlWindow = 12

// refreshDownloadItems recomputes the filtered Nerd Font catalog and keeps
// the cursor on a real row.
func (m *Model) refreshDownloadItems() {
	q := strings.ToLower(strings.TrimSpace(m.dlFilter.Value()))
	out := make([]string, 0, len(m.dlNames))
	for _, n := range m.dlNames {
		if q == "" || strings.Contains(strings.ToLower(n), q) {
			out = append(out, n)
		}
	}
	m.dlFiltered = out
	if m.dlCursor >= len(out) {
		m.dlCursor = max(len(out)-1, 0)
	}
	if m.dlCursor < 0 {
		m.dlCursor = 0
	}
	m.clampDownloadWindow()
}

// clampDownloadWindow slides the visible window so the cursor always sits
// inside it without leaving a partial page at the end.
func (m *Model) clampDownloadWindow() {
	if m.dlCursor < m.dlOffset {
		m.dlOffset = m.dlCursor
	}
	if m.dlCursor >= m.dlOffset+dlWindow {
		m.dlOffset = m.dlCursor - dlWindow + 1
	}
	if m.dlOffset > len(m.dlFiltered)-dlWindow {
		m.dlOffset = len(m.dlFiltered) - dlWindow
	}
	if m.dlOffset < 0 {
		m.dlOffset = 0
	}
}

// dismissDownload closes the download overlay, orphaning any in-flight
// fetch so its late completion refreshes quietly.
func (m *Model) dismissDownload() (Model, tea.Cmd) {
	if m.dlActive {
		m.dlGen++
		m.dlActive = false
		m.dlProgress = 0
		m.dlKnown = false
		m.overlay = overlayNone
		m.sizeWidgets()
		m.status = "Download dismissed — finishing in background"
		return *m, nil
	}
	m.overlay = overlayNone
	m.sizeWidgets()
	m.status = "Download cancelled"
	return *m, nil
}

// fetchFont is a package-level var (rather than a direct
// downloader.Fetch call) so tests can run downloadStartMsg without
// touching the network — same pattern as listLibrary/restoreOriginal.
var fetchFont = downloader.Fetch

// startFetch runs a Nerd Font download off the Elm loop: the returned
// channel streams downloadProgressMsg values as body bytes arrive and
// closes after the final downloadDoneMsg. The returned Cmd delivers the
// first message; Update re-arms it for the rest while the download is
// current. Progress travels through the channel, so the fetch goroutine
// never touches model state.
func startFetch(name string, gen int) (chan tea.Msg, tea.Cmd) {
	ch := make(chan tea.Msg, 8)
	go func() {
		defer close(ch)
		progress := func(done, total int64) {
			if total <= 0 {
				ch <- downloadProgressMsg{known: false, gen: gen}
				return
			}
			ch <- downloadProgressMsg{frac: float64(done) / float64(total), gen: gen, known: true}
		}
		dest, err := fetchFont(name, false, progress)
		ch <- downloadDoneMsg{path: dest, err: err, gen: gen}
	}()
	return ch, func() tea.Msg { return <-ch } // first message; Update re-arms for the rest
}
