package tui

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/GeneralKaos666/font-changer-termux/internal/importer"
)

type importDoneMsg struct {
	path string
	err  error
}

// importCmd copies an external font into the library as a message.
// clash is one of "ask", "error", "keep-both", "replace"; a
// *importer.ClashError carries the *source* path in the message (not the
// colliding destination) so the clash prompt can retry the import with
// the policy the user picks.
func importCmd(path, clash string) tea.Cmd {
	return func() tea.Msg {
		if strings.TrimSpace(path) == "" {
			return importDoneMsg{err: fmt.Errorf("type a font file path first")}
		}
		dest, err := importer.ImportFile(path, clash)
		var ce *importer.ClashError
		if errors.As(err, &ce) {
			return importDoneMsg{path: path, err: err}
		}
		return importDoneMsg{path: dest, err: err}
	}
}

// expandHome rewrites a leading "~" to the user's home directory; any
// other path (absolute, relative, "~user") passes through unchanged, as
// does everything when $HOME is unavailable.
func expandHome(p string) string {
	if p == "~" || strings.HasPrefix(p, "~/") {
		if home, err := os.UserHomeDir(); err == nil && home != "" {
			if p == "~" {
				return home
			}
			return filepath.Join(home, p[2:])
		}
	}
	return p
}

// completePath completes s for a Tab press: "~" expands first, an
// existing directory gains a trailing separator, and otherwise the last
// segment completes to the single matching entry below dir, or to the
// longest common prefix shared by several matches. When nothing can be
// completed the input is returned unchanged.
func completePath(s string) string {
	s = expandHome(s)
	if s == "" {
		return s
	}
	if fi, err := os.Stat(s); err == nil && fi.IsDir() {
		if !strings.HasSuffix(s, string(filepath.Separator)) {
			return s + string(filepath.Separator)
		}
		return s
	}
	dir, base := filepath.Split(s)
	if dir == "" {
		return s // bare relative name: nothing to read below
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return s
	}
	matches := make([]string, 0, len(entries))
	for _, e := range entries {
		if strings.HasPrefix(strings.ToLower(e.Name()), strings.ToLower(base)) {
			matches = append(matches, e.Name())
		}
	}
	if len(matches) == 1 {
		return dir + matches[0]
	}
	lcp := longestCommonPrefix(matches, base)
	if lcp == "" {
		return s // no match
	}
	return dir + lcp
}

// longestCommonPrefix returns the longest prefix shared by names,
// never shorter than the typed base (every name matches it
// case-insensitively, so a case difference must not eat user input);
// empty when there are no names at all.
func longestCommonPrefix(names []string, base string) string {
	if len(names) == 0 {
		return ""
	}
	lcp := names[0]
	for _, n := range names[1:] {
		i := 0
		for i < len(lcp) && i < len(n) && lcp[i] == n[i] {
			i++
		}
		lcp = lcp[:i]
	}
	if len(lcp) < len(base) {
		return base
	}
	return lcp
}
