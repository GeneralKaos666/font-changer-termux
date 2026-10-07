// Package theme reads the Termux color palette so the TUI can dress
// itself in the user's terminal colors. Anything missing degrades to
// "" (caller keeps its defaults).
package theme

import (
	"bufio"
	"os"
	"regexp"
	"strings"
)

var hexColor = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

// Palette holds the colors.properties values the TUI themes itself with.
// Empty strings mean "not set".
type Palette struct {
	Background string
	Foreground string
	Cursor     string
	Accent     string // color12, fallback color4
	Muted      string // color8
}

// LoadFile parses a colors.properties file. A missing file is an error;
// malformed lines are ignored.
func LoadFile(path string) (Palette, error) {
	f, err := os.Open(path)
	if err != nil {
		return Palette{}, err
	}
	defer f.Close()
	raw := map[string]string{}
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key, val = strings.TrimSpace(key), strings.TrimSpace(val)
		if hexColor.MatchString(val) {
			raw[key] = val
		}
	}
	if err := sc.Err(); err != nil {
		return Palette{}, err
	}
	p := Palette{
		Background: raw["background"],
		Foreground: raw["foreground"],
		Cursor:     raw["cursor"],
		Muted:      raw["color8"],
	}
	p.Accent = raw["color12"]
	if p.Accent == "" {
		p.Accent = raw["color4"]
	}
	return p, nil
}
