package tui

import (
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/mattn/go-runewidth"

	"github.com/GeneralKaos666/nerdfont-changer/internal/paths"
	"github.com/GeneralKaos666/nerdfont-changer/internal/scan"
	"github.com/GeneralKaos666/nerdfont-changer/internal/theme"
)

// Layout thresholds, in terminal cells.
const (
	minW = 44 // hard floor: narrower than this and View shows a notice
	minH = 8  // hard floor: shorter than this and View shows a notice

	// Vertical split budget. The band (title/status/help excluded) is shared
	// by the preview on top and the active lower pane below (library list, or
	// the Nerd Font picker while downloading).
	previewNatural = 12 // preferred preview content lines
	previewMin     = 4  // fold the preview below this many content lines
	lowerMin       = 6  // lower pane outer floor: border + filter + three rows
	splitPercent   = 45 // preview's share of a band shorter than it would like
)

// asciiMode swaps the box borders, titled rules, selection marker, applied
// badge, filter glyph, dotted rule, ellipsis and prompt/coverage samples for
// plain ASCII. It is off unless SetASCII says otherwise, so Unicode
// terminals keep the rounded look.
var asciiMode bool

// SetASCII selects the plain-ASCII chrome. Call it before NewModel so the
// derived (themed) styles pick up the ASCII border.
func SetASCII(on bool) { asciiMode = on }

// chrome helpers resolve the few glyphs that differ per rendering mode.
func borderFor() lipgloss.Border {
	if asciiMode {
		return lipgloss.Border{
			Top: "-", Bottom: "-", Left: "|", Right: "|",
			TopLeft: "+", TopRight: "+", BottomLeft: "+", BottomRight: "+",
		}
	}
	return lipgloss.RoundedBorder()
}

func ellipsis() string {
	if asciiMode {
		return "..."
	}
	return "…"
}

func markerGlyph() string {
	if asciiMode {
		return "> "
	}
	return "▶ "
}

func badgeDot() string {
	if asciiMode {
		return "*"
	}
	return "●"
}

// filterGlyph labels the search inputs.
func filterGlyph() string {
	if asciiMode {
		return "? "
	}
	return "⌕ "
}

// ruleGlyph is the horizontal separator character.
func ruleGlyph() string {
	if asciiMode {
		return "-"
	}
	return "┄"
}

// coverageLine is the Nerd/powerline sample row.
func coverageLine() string {
	if asciiMode {
		return "<> [] {} ^ -> ok"
	}
	return "\ue0a0 \ue0a1 \ue0a2 \u03b3 \u03bb \u2211 \u2192 \u2713"
}

// mockPromptLines is the fallback shell-prompt sample, used when the live
// capture is unavailable.
func mockPromptLines() []string {
	if asciiMode {
		return []string{"+--[user host]--[~/demo] main", `\--$ AaBbCcDdEe`}
	}
	return []string{"╭─[user 󰀲 host]─[~/demo] main", "╰─❯ AaBbCcDdEe"}
}

// slotChip renders the active slot as an accent chip.
func slotChip(slot string) string {
	open, close := "⟨", "⟩"
	if asciiMode {
		open, close = "<", ">"
	}
	return helpKeyStyle.Render(open + " " + slot + " " + close)
}

// Restrained palette: accent + muted + default foreground only.
var (
	accent = lipgloss.AdaptiveColor{Light: "#5B50E6", Dark: "#A49EFF"}
	muted  = lipgloss.AdaptiveColor{Light: "#6B7280", Dark: "#8E8E96"}

	slotStyle     lipgloss.Style
	boxStyle      lipgloss.Style
	borderStyle   lipgloss.Style
	statusStyle   lipgloss.Style
	helpStyle     lipgloss.Style
	helpKeyStyle  lipgloss.Style
	badgeStyle    lipgloss.Style
	sampleStyle   lipgloss.Style
	promptStyle   lipgloss.Style
	metaStyle     lipgloss.Style
	ruleStyle     lipgloss.Style
	matchStyle    lipgloss.Style
	selectedStyle lipgloss.Style
)

func init() { applyTheme(theme.Palette{}) }

// applyTheme re-points the palette and rebuilds every derived style.
// Empty palette fields keep the built-in defaults, so a missing or
// partial colors.properties never breaks the look.
func applyTheme(p theme.Palette) {
	if p.Accent != "" {
		accent = lipgloss.AdaptiveColor{Light: p.Accent, Dark: p.Accent}
	}
	if p.Muted != "" {
		muted = lipgloss.AdaptiveColor{Light: p.Muted, Dark: p.Muted}
	}
	fg := lipgloss.NoColor{}
	var fgColor lipgloss.TerminalColor = fg
	if p.Foreground != "" {
		fgColor = lipgloss.Color(p.Foreground)
	}
	slotStyle = lipgloss.NewStyle().Foreground(muted)
	boxStyle = lipgloss.NewStyle().Border(borderFor()).BorderForeground(accent).Padding(0, 1)
	borderStyle = lipgloss.NewStyle().Foreground(accent)
	statusStyle = lipgloss.NewStyle().Padding(0, 1).Foreground(fgColor)
	helpStyle = lipgloss.NewStyle().Foreground(muted)
	helpKeyStyle = lipgloss.NewStyle().Foreground(accent).Bold(true)
	sampleStyle = lipgloss.NewStyle().Bold(true).Foreground(fgColor)
	promptStyle = lipgloss.NewStyle().Foreground(muted)
	metaStyle = lipgloss.NewStyle().Foreground(muted)
	ruleStyle = lipgloss.NewStyle().Foreground(muted)
	matchStyle = lipgloss.NewStyle().Foreground(accent).Bold(true)
	selectedStyle = lipgloss.NewStyle().Background(accent).Foreground(lipgloss.Color("#FFFFFF")).Bold(true)
	badgeStyle = lipgloss.NewStyle().Foreground(accent)
}

// fontItem adapts scan.FontEntry to the bubbles list.
type fontItem struct {
	entry scan.FontEntry
	badge string
}

// FilterValue is the value we use when filtering against this item.
func (i fontItem) FilterValue() string { return i.entry.Name }

// fontDelegate renders one row with the current filter query highlighted.
type fontDelegate struct {
	query string
	width int // inner content width; rows truncate to it (0 = no clamp)
}

// Height is the height of the list item.
func (d fontDelegate) Height() int { return 2 }

// Spacing is the gap between list items. Rows are two lines tall and the
// vertical layout is tight, so no extra inter-item blank is spent.
func (d fontDelegate) Spacing() int { return 0 }

// Update is the update loop for items; rows are static.
func (d fontDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }

// Render renders one row, accent-highlighting the filter match, marking
// the selected row with a cursor glyph and giving it an accent background.
func (d fontDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	fi, ok := item.(fontItem)
	if !ok {
		return
	}
	selected := index == m.Index()
	// Truncate before styling: keeps cell widths exact and avoids
	// cutting styled output mid-escape (color bleed). The selected row
	// reserves room for its marker so the row never overflows, and the
	// marker (unlike color) survives monochrome terminals.
	marker := ""
	if selected {
		marker = markerGlyph()
	}
	name := fi.entry.Name
	if d.width > 0 {
		room := d.width - runewidth.StringWidth(marker)
		if fi.badge != "" {
			room -= runewidth.StringWidth(fi.badge) + 1
		}
		name = truncateCells(name, room)
	}
	title := highlightMatch(name, d.query)
	descPlain := entrySummary(fi.entry)
	if d.width > 0 {
		descPlain = truncateCells(descPlain, d.width)
	}
	desc := lipgloss.NewStyle().Foreground(muted).Render(descPlain)
	if fi.badge != "" {
		title += " " + badgeStyle.Render(fi.badge)
	}
	if selected {
		title = selectedStyle.Render(marker + name)
		desc = selectedStyle.Render(descPlain)
		if fi.badge != "" {
			title += " " + selectedStyle.Render(fi.badge)
		}
	}
	fmt.Fprintf(w, "%s\n%s", title, desc)
}

// truncateCells cuts s to maxW terminal cells (CJK/emoji aware,
// ANSI-escape aware), appending an ellipsis when shortened.
func truncateCells(s string, maxW int) string {
	if maxW <= 0 {
		return ""
	}
	return runewidth.Truncate(s, maxW, ellipsis())
}

// highlightMatch bolds the first case-insensitive occurrence of query.
func highlightMatch(name, query string) string {
	q := strings.TrimSpace(query)
	if q == "" {
		return name
	}
	lower, lq := strings.ToLower(name), strings.ToLower(q)
	at := strings.Index(lower, lq)
	if at < 0 {
		return name
	}
	return name[:at] + matchStyle.Render(name[at:at+len(q)]) + name[at+len(q):]
}

func humanSize(n int64) string {
	if n < 1024 {
		return fmt.Sprintf("%d B", n)
	}
	return fmt.Sprintf("%.0f KB", float64(n)/1024)
}

// entrySummary renders the "Family · Style · Size" descriptor line.
func entrySummary(e scan.FontEntry) string {
	return fmt.Sprintf("%s · %s · %s", e.Family, e.Style, humanSize(e.Size))
}

// boxInnerWidth is the content width inside a pane box.
func (m Model) boxInnerWidth() int {
	if m.contentW > 0 {
		return m.contentW
	}
	w := m.width
	if w <= 0 {
		w = 96
	}
	return max(w-4, 10)
}

// previewBox wraps the preview in its titled pane, or "" when the terminal
// is too short to afford it.
func (m Model) previewBox() string {
	if m.previewH <= 0 {
		return ""
	}
	return titledBox("Preview", m.PreviewPane(), m.contentW)
}

// PreviewPane renders the rich preview: a name/style header, a large sample
// with Nerd/powerline coverage, an aligned metadata grid (glyphs, UPM,
// version, slot, file, backup) and the live shell prompt. Every line is
// truncated to the box width (cell-aware) and padded to the target height.
func (m Model) PreviewPane() string {
	inner := m.boxInnerWidth()
	e, ok := m.selectedEntry()
	name, style := "—", ""
	if ok {
		name, style = e.Name, e.Style
	}
	lines := []string{
		spread(sampleStyle.Render(truncateCells(name, max(inner-10, 8))), slotStyle.Render(truncateCells(style, 8)), inner),
		rule(inner),
		sampleStyle.Render(truncateCells("AaBbCc 0123456789", inner)),
		truncateCells("AaBbCcDdEeFfGg 0123456789 !?#@%", inner),
		truncateCells(coverageLine(), inner),
		rule(inner),
	}
	if !ok {
		lines = append(lines, "", "No font selected.")
	} else {
		backup := "none"
		if m.state != nil && m.state.BackedUp[m.slot] {
			backup = "taken"
		}
		ver := "—"
		if m.hasDetail && m.detail.Version != "" {
			ver = m.detail.Version
		}
		glyphs, upm := "—", "—"
		if m.hasDetail {
			glyphs = fmt.Sprintf("%d", m.detail.Glyphs)
			upm = fmt.Sprintf("%d", m.detail.UPM)
		}
		lines = append(lines,
			metaLine(inner, "glyphs", glyphs, "UPM", upm),
			metaLine(inner, "version", ver, "slot", m.slot),
			metaLine(inner, "file", paths.SlotFiles[m.slot], "backup", backup),
		)
	}
	if m.Dirty() {
		lines = append(lines, promptStyle.Render(truncateCells("preview — Enter keeps, Esc restores", inner)))
	}
	// Live shell prompt when captured (raw ANSI passes through, so it renders
	// pixel-for-pixel), mock fallback otherwise. Either way the terminal's
	// live font shows whether prompt glyphs survive the previewed typeface.
	if len(m.prompt) > 0 {
		for _, ln := range m.prompt {
			// Re-append a reset: truncation may cut the line's own one.
			lines = append(lines, truncateCells(ln, inner)+"\x1b[0m")
		}
	} else {
		for _, ln := range mockPromptLines() {
			lines = append(lines, promptStyle.Render(truncateCells(ln, inner)))
		}
	}
	for len(lines) < m.previewH {
		lines = append(lines, "")
	}
	if m.previewH > 0 && len(lines) > m.previewH {
		lines = lines[:m.previewH]
	}
	return strings.Join(lines, "\n")
}

// spread lays left and right on one line, right flush to width.
func spread(left, right string, width int) string {
	gap := width - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 1 {
		return clampLine(left, width)
	}
	return left + strings.Repeat(" ", gap) + right
}

// padCells right-pads a plain string to width cells.
func padCells(s string, width int) string {
	if w := runewidth.StringWidth(s); w < width {
		return s + strings.Repeat(" ", width-w)
	}
	return s
}

// padStyled right-pads an ANSI-styled string to width cells.
func padStyled(s string, width int) string {
	if w := lipgloss.Width(s); w < width {
		return s + strings.Repeat(" ", width-w)
	}
	return s
}

// rule draws a full-width dim separator.
func rule(inner int) string {
	if inner <= 0 {
		return ""
	}
	return ruleStyle.Render(strings.Repeat(ruleGlyph(), inner))
}

// metaLine lays two "label value" pairs into two aligned columns.
func metaLine(inner int, lLabel, lVal, rLabel, rVal string) string {
	const labelW = 7
	colW := max(inner/2, labelW+2)
	cell := func(label, val string) string {
		return metaStyle.Render(padCells(label, labelW)) + " " +
			truncateCells(val, max(colW-labelW-1, 1))
	}
	return padStyled(cell(lLabel, lVal), colW) + cell(rLabel, rVal)
}

// gradientTitle renders the title with a restrained two-stop gradient.
func gradientTitle(s string) string {
	const from = "#C9C2FF"
	const to = "#5B50E6"
	runes := []rune(s)
	var b strings.Builder
	for i, r := range runes {
		t := 0.0
		if len(runes) > 1 {
			t = float64(i) / float64(len(runes)-1)
		}
		c := lipgloss.Color(blendHex(from, to, t))
		b.WriteString(lipgloss.NewStyle().Foreground(c).Bold(true).Render(string(r)))
	}
	return b.String()
}

func blendHex(a, b string, t float64) string {
	var ar, ag, ab, br, bg, bb int
	fmt.Sscanf(a, "#%02x%02x%02x", &ar, &ag, &ab)
	fmt.Sscanf(b, "#%02x%02x%02x", &br, &bg, &bb)
	mix := func(x, y int) int { return int(float64(x)*(1-t) + float64(y)*t) }
	return fmt.Sprintf("#%02X%02X%02X", mix(ar, br), mix(ag, bg), mix(ab, bb))
}

// helpBar renders the context-sensitive key hints: the footer always
// advertises exactly the keys that work in the current overlay/focus,
// instead of one static wall of shortcuts.
func (m Model) helpBar() string {
	var keys []string
	switch {
	case m.overlay == overlayImportClash:
		keys = []string{"1/i keep both", "2/r replace", "esc cancel"}
	case m.overlay == overlayImport:
		keys = []string{"enter import", "tab complete", "esc cancel"}
	case m.overlay == overlayDownload && m.dlFilterFocused:
		keys = []string{"type to filter", "esc done"}
	case m.overlay == overlayDownload:
		keys = []string{"↑/↓ choose", "tab filter", "enter download", "esc cancel"}
	case m.focusFilter:
		keys = []string{"type to search", "enter done", "esc clear"}
	default:
		keys = []string{"space/p try", "enter keep", "s slot", "i import", "d download", "esc undo", "q quit"}
	}
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		if i := strings.Index(k, " "); i >= 0 {
			parts = append(parts, helpKeyStyle.Render(k[:i])+helpStyle.Render(k[i:]))
		} else {
			parts = append(parts, helpKeyStyle.Render(k))
		}
	}
	return helpStyle.Render(strings.Join(parts, " · "))
}

// View renders the frame: title, the library/preview (or download) panes,
// overlays, status and the context-sensitive help bar — all TTY-free.
// Every line is clamped to the terminal width, so nothing can wrap.
func (m Model) View() string {
	if m.contentW == 0 {
		m.sizeWidgets()
	}
	w, h := m.width, m.height
	if w <= 0 {
		w = 96
	}
	if h <= 0 {
		h = 28
	}
	if m.tooSmall {
		return tooSmallView(w, h)
	}

	title := spread(" "+gradientTitle("nerdfont-changer"), slotChip(m.slot)+" ", w)

	body := m.libraryLayout()
	if m.overlay == overlayDownload {
		body = m.downloadLayout()
	}

	var b strings.Builder
	b.WriteString(clampLine(title, w) + "\n")
	b.WriteString(body + "\n")

	if m.overlay == overlayImport {
		b.WriteString(boxStyle.Width(max(w-2, 30)).Render("Import font path:\n"+m.importInput.View()) + "\n")
	}
	if m.overlay == overlayImportClash {
		msg := "File already exists: " + filepath.Base(m.pendingImportPath)
		b.WriteString(boxStyle.Width(max(w-2, 30)).Render(truncateCells(msg, max(w-8, 10))) + "\n")
	}

	status := m.status
	if status == "" {
		status = "—"
	}
	b.WriteString(clampLine(statusStyle.Render(status), w) + "\n")
	b.WriteString(clampLine(m.helpBar(), w))
	return b.String()
}

// clampLine truncates a (possibly styled) line to at most w terminal cells,
// appending an ellipsis. It is ANSI-aware, so escape sequences survive.
func clampLine(s string, w int) string {
	if w <= 0 || lipgloss.Width(s) <= w {
		return s
	}
	return ansi.Truncate(s, w, ellipsis())
}

// tooSmallView is the floor state: a centered notice naming the minimum
// size, in place of a mangled frame.
func tooSmallView(w, h int) string {
	msg := fmt.Sprintf("Terminal too small — need %d×%d, have %d×%d", minW, minH, w, h)
	return lipgloss.Place(w, max(h, 1), lipgloss.Center, lipgloss.Center, clampLine(msg, w))
}

// libraryLayout is the default frame: preview on top, library list below.
func (m Model) libraryLayout() string {
	return paneStack(m.previewBox(), m.listBox())
}

// downloadLayout swaps the lower pane while the Nerd Font picker is open;
// the preview stays on top.
func (m Model) downloadLayout() string {
	return paneStack(m.previewBox(), m.downloadBox())
}

// paneStack joins non-empty panes top-to-bottom with no gap.
func paneStack(panes ...string) string {
	parts := make([]string, 0, len(panes))
	for _, p := range panes {
		if strings.TrimSpace(p) != "" {
			parts = append(parts, p)
		}
	}
	return strings.Join(parts, "\n")
}

// listBox renders the library filter line plus the scrollable list.
func (m Model) listBox() string {
	header := spread(m.filter.View(), statusStyle.Render(fmt.Sprintf("%d fonts", len(m.VisibleEntries()))), m.contentW)
	content := header + "\n"
	if len(m.list.Items()) > 0 {
		content += m.list.View()
	} else {
		content += statusStyle.Render("No fonts — press i to import, d to download.")
	}
	return titledBox("Library", fitLines(content, m.listRows+1), m.contentW)
}

// fitLines pads or truncates a block to exactly n lines.
func fitLines(s string, n int) string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	for len(lines) < n {
		lines = append(lines, "")
	}
	if len(lines) > n {
		lines = lines[:n]
	}
	return strings.Join(lines, "\n")
}

// downloadBox renders the windowed Nerd Font picker, padded to the pane
// height so the frame never overflows.
func (m Model) downloadBox() string {
	shown := 0
	if len(m.dlFiltered) > 0 {
		shown = m.dlCursor + 1
	}
	var b strings.Builder
	b.WriteString(spread(m.dlFilter.View(), statusStyle.Render(fmt.Sprintf("%d/%d", shown, len(m.dlFiltered))), m.contentW) + "\n")
	end := min(m.dlOffset+m.dlWindowRows(), len(m.dlFiltered))
	for i := m.dlOffset; i < end; i++ {
		line := "  " + truncateCells(m.dlFiltered[i], max(m.contentW-2, 4))
		if i == m.dlCursor {
			line = selectedStyle.Render(markerGlyph() + truncateCells(m.dlFiltered[i], max(m.contentW-4, 4)))
		}
		b.WriteString(line + "\n")
	}
	if len(m.dlFiltered) == 0 {
		b.WriteString(statusStyle.Render("  no matches") + "\n")
	}
	if m.dlActive {
		line := " Downloading " + m.dlName + "…"
		if !m.dlKnown {
			line += " (no progress info)"
		}
		b.WriteString("\n" + clampLine(m.spinner.View()+line, m.contentW) + "\n")
		b.WriteString(m.progress.ViewAs(m.dlProgress) + "\n")
	}
	return titledBox("Nerd Fonts", fitLines(b.String(), m.listRows+1), m.contentW)
}

// titledBox renders a full-width bordered pane with its title embedded in
// the top border. content must already be fitted to the pane's inner height.
// width is the pane's text width; lipgloss counts horizontal padding toward
// Style.Width, so the body style asks for width+2 (plus two border cells).
func titledBox(title, content string, width int) string {
	body := boxStyle.BorderTop(false).Width(width + 2).Render(content)
	return titleRule(title, width+4) + "\n" + body
}

// titleRule draws the top border line with a left-aligned title.
func titleRule(title string, outer int) string {
	left, fill, right := "╭─", "─", "╮"
	if asciiMode {
		left, fill, right = "+-", "-", "+"
	}
	// Widths are display cells, not bytes: the box characters are multibyte.
	lw, rw := lipgloss.Width(left), lipgloss.Width(right)
	if outer < lw+rw {
		outer = lw + rw
	}
	if title == "" {
		return borderStyle.Render(left + strings.Repeat(fill, outer-lw-rw) + right)
	}
	pad := outer - lw - lipgloss.Width(title) - 3 // " title " plus the corners
	if pad < 0 {
		pad = 0
	}
	return borderStyle.Render(left + " " + title + " " + strings.Repeat(fill, pad) + right)
}

// sizeWidgets computes the vertical split. Title, status and help reserve
// three rows; the rest is the band, shared by the preview on top and the
// active lower pane (library list or download picker) below.
func (m *Model) sizeWidgets() {
	w, h := m.width, m.height
	if w <= 0 {
		w = 96
	}
	if h <= 0 {
		h = 28
	}
	m.tooSmall = w < minW || h < minH

	m.band = 28 // unsized default
	if m.height > 0 {
		m.band = max(m.height-3, 4)
	}
	m.contentW = max(w-4, 10)

	// Give the preview its natural height, but never squeeze the lower pane
	// below lowerMin, and never let the preview exceed splitPercent of the
	// band. Below previewMin lines, fold the preview away entirely.
	previewOuter := previewNatural + 2
	if want := m.band - lowerMin; previewOuter > want {
		previewOuter = want
	}
	if capped := m.band * splitPercent / 100; previewOuter > capped {
		previewOuter = capped
	}
	if previewOuter < previewMin+2 {
		previewOuter = 0
	}
	m.folded = previewOuter == 0
	m.previewH = max(previewOuter-2, 0)
	m.listRows = max(m.band-previewOuter-2-1, 1) // lower pane: minus border and filter line

	m.filter.Width = max(m.contentW-10, 8)
	m.dlFilter.Width = max(m.contentW-10, 8)
	m.list.SetSize(m.contentW, m.listRows)
	m.delegate.width = m.contentW
	m.list.SetDelegate(m.delegate)
	m.progress.Width = max(m.contentW, 20)
}
