package tui

import (
	"fmt"
	"io"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-runewidth"

	"termux-fonts-go/internal/paths"
	"termux-fonts-go/internal/scan"
	"termux-fonts-go/internal/theme"
)

// Restrained palette: accent + muted + default foreground only.
var (
	accent = lipgloss.AdaptiveColor{Light: "#5B50E6", Dark: "#A49EFF"}
	muted  = lipgloss.AdaptiveColor{Light: "#6B7280", Dark: "#8E8E96"}

	titleStyle    lipgloss.Style
	slotStyle     lipgloss.Style
	boxStyle      lipgloss.Style
	statusStyle   lipgloss.Style
	helpStyle     lipgloss.Style
	helpKeyStyle  lipgloss.Style
	badgeStyle    lipgloss.Style
	sampleStyle   lipgloss.Style
	promptStyle   lipgloss.Style
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
	titleStyle = lipgloss.NewStyle().Bold(true).Padding(0, 1).Foreground(fgColor)
	slotStyle = lipgloss.NewStyle().Foreground(muted)
	boxStyle = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(accent).Padding(0, 1)
	statusStyle = lipgloss.NewStyle().Padding(0, 1).Foreground(fgColor)
	helpStyle = lipgloss.NewStyle().Foreground(muted)
	helpKeyStyle = lipgloss.NewStyle().Foreground(accent).Bold(true)
	sampleStyle = lipgloss.NewStyle().Bold(true).Foreground(fgColor)
	promptStyle = lipgloss.NewStyle().Foreground(muted)
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

// Spacing is the gap between list items.
func (d fontDelegate) Spacing() int { return 1 }

// Update is the update loop for items; rows are static.
func (d fontDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }

// Render renders one row, accent-highlighting the filter match and giving
// the selected row an accent background.
func (d fontDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	fi, ok := item.(fontItem)
	if !ok {
		return
	}
	// Truncate before styling: keeps cell widths exact and avoids
	// cutting styled output mid-escape (color bleed).
	name := fi.entry.Name
	if d.width > 0 {
		room := d.width
		if fi.badge != "" {
			room -= runewidth.StringWidth(fi.badge) + 1
		}
		name = truncateCells(name, room)
	}
	title := highlightMatch(name, d.query)
	descPlain := fmt.Sprintf("%s · %s · %s", fi.entry.Family, fi.entry.Style, humanSize(fi.entry.Size))
	if d.width > 0 {
		descPlain = truncateCells(descPlain, d.width)
	}
	desc := lipgloss.NewStyle().Foreground(muted).Render(descPlain)
	if fi.badge != "" {
		title += " " + badgeStyle.Render(fi.badge)
	}
	if index == m.Index() {
		title = selectedStyle.Render(name)
		desc = selectedStyle.Render(descPlain)
		if fi.badge != "" {
			title += " " + selectedStyle.Render(fi.badge)
		}
	}
	fmt.Fprintf(w, "%s\n%s", title, desc)
}

// truncateCells cuts s to maxW terminal cells (CJK/emoji aware,
// ANSI-escape aware), appending "…" when shortened.
func truncateCells(s string, maxW int) string {
	if maxW <= 0 {
		return ""
	}
	return runewidth.Truncate(s, maxW, "…")
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

// boxInnerWidth is the content width inside a full-width bordered box.
func (m Model) boxInnerWidth() int {
	w := m.width
	if w <= 0 {
		w = 96
	}
	return max(max(w-4, 30)-4, 10)
}

// PreviewPane renders the rich preview: large sample, Nerd/powerline
// coverage row, file info and slot + backup status. Every line is
// truncated to the box width (cell-aware) and, when the model carries a
// target preview height, padded to fill it exactly.
func (m Model) PreviewPane() string {
	inner := m.boxInnerWidth()
	slotFile := paths.SlotFiles[m.slot]
	backup := "no backup yet"
	if m.state != nil && m.state.BackedUp[m.slot] {
		backup = "backup taken"
	}
	preview := "—"
	if e, ok := m.selectedEntry(); ok {
		preview = e.Name
	}
	slotLine := fmt.Sprintf("%s ← %s • %s", slotFile, preview, backup)
	if m.Dirty() {
		slotLine += " • preview — Enter keeps, Esc restores"
	}
	raw := []string{truncateCells(slotLine, inner), "AaBbCc 0123456789", "AaBbCcDdEeFfGg 0123456789 !?#@%", "Nerd/powerline coverage:", "  \ue0a0 \ue0a1 \ue0a2 \u03b3 \u03bb \u2211 \u2192 \u2713  "}
	lines := []string{sampleStyle.Render(truncateCells(raw[1], inner))}
	for _, ln := range raw[2:] {
		lines = append(lines, truncateCells(ln, inner))
	}
	lines = append([]string{truncateCells(raw[0], inner)}, lines...)

	e, ok := m.selectedEntry()
	if !ok {
		lines = append(lines, "", "No font selected.")
	} else {
		lines = append(lines, "", truncateCells(e.Name, inner),
			truncateCells(fmt.Sprintf("%s · %s · %s", e.Family, e.Style, humanSize(e.Size)), inner))
		if d, err := scan.Describe(e.Path); err == nil {
			ver := d.Version
			if ver == "" {
				ver = "—"
			}
			lines = append(lines, truncateCells(fmt.Sprintf("%d glyphs · %d UPM · %s", d.Glyphs, d.UPM, ver), inner))
		}
	}
	// Live shell prompt when captured (raw ANSI passes through, so it
	// renders pixel-for-pixel), mock fallback otherwise. Either way the
	// terminal's live font shows whether prompt glyphs survive the
	// previewed typeface.
	if len(m.prompt) > 0 {
		for _, ln := range m.prompt {
			// Re-append a reset: truncation may cut the line's own one.
			lines = append(lines, truncateCells(ln, inner)+"\x1b[0m")
		}
	} else {
		lines = append(lines, promptStyle.Render(truncateCells("╭─[user 󰀲 host]─[~/demo] main", inner)))
		lines = append(lines, promptStyle.Render(truncateCells("╰─❯ AaBbCcDdEe", inner)))
	}
	lines = append(lines, "")
	for len(lines) < m.previewH {
		lines = append(lines, "")
	}
	if m.previewH > 0 && len(lines) > m.previewH {
		lines = lines[:m.previewH]
	}
	return strings.Join(lines, "\n")
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

func helpBar() string {
	keys := []string{"space/p preview", "enter keep", "s slot", "i import", "d download", "esc restore", "q quit", "tab filter"}
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

// View renders title, preview (top half) + filter/list (bottom half),
// overlays, status and help — all TTY-free.
func (m Model) View() string {
	w := m.width
	if w <= 0 {
		w = 96
	}

	title := titleStyle.Render(gradientTitle("termux-fonts")) + slotStyle.Render("slot: "+m.slot)
	filterLine := m.filter.View() + statusStyle.Render(fmt.Sprintf("  %d/%d", len(m.VisibleEntries()), len(m.entries)))

	preview := boxStyle.Width(max(w-4, 30)).Render(m.PreviewPane())

	listContent := statusStyle.Render("No fonts in library — press i to import, d to download.")
	if len(m.list.Items()) > 0 {
		listContent = m.list.View()
	}
	fonts := boxStyle.Width(max(w-4, 30)).Render(listContent)

	var b strings.Builder
	b.WriteString(title + "\n")
	b.WriteString(preview + "\n")
	b.WriteString(filterLine + "\n")
	b.WriteString(fonts + "\n")

	if m.overlay == overlayImport {
		b.WriteString(boxStyle.Render("Import font path:\n"+m.importInput.View()) + "\n")
	}
	if m.overlay == overlayDownload {
		var names strings.Builder
		for i, name := range m.dlNames {
			cursor := "  "
			if i == m.dlCursor {
				cursor = "> "
			}
			line := cursor + name
			if i == m.dlCursor {
				line = selectedStyle.Render(line)
			}
			names.WriteString(line + "\n")
		}
		b.WriteString(boxStyle.Render("Nerd Fonts (↑/↓ + Enter):\n"+names.String()) + "\n")
	}
	if m.dlActive {
		bar := m.progress.ViewAs(m.dlProgress)
		b.WriteString(boxStyle.Render(fmt.Sprintf("%s Downloading %s… (no progress info)\n%s",
			m.spinner.View(), m.dlName, bar)) + "\n")
	}

	status := m.status
	if status == "" {
		status = "—"
	}
	b.WriteString(statusStyle.Render(status) + "\n")
	b.WriteString(helpBar())
	return b.String()
}
