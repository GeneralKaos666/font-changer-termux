// Package scan lists the font library and reads the active Termux font slots.
package scan

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"golang.org/x/image/font/sfnt"

	"termux-fonts-go/internal/paths"
)

// FontEntry is one font file in the user's font library.
type FontEntry struct {
	Name   string
	Path   string
	Size   int64
	Mtime  time.Time
	Family string
	Style  string
}

// FontDetails carries display metadata parsed from a font's tables.
type FontDetails struct {
	Glyphs  int
	UPM     int32
	Version string
}

// Describe parses tables for display metadata. Garbage in, error out.
func Describe(path string) (FontDetails, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return FontDetails{}, err
	}
	font, err := sfnt.Parse(data)
	if err != nil {
		return FontDetails{}, err
	}
	var buf sfnt.Buffer
	version, _ := font.Name(&buf, sfnt.NameIDVersion)
	return FontDetails{
		Glyphs:  font.NumGlyphs(),
		UPM:     int32(font.UnitsPerEm()),
		Version: version,
	}, nil
}

// familyStyle returns (family, style) from the font's name table,
// preferring typographic names (IDs 16/17) over legacy ones (IDs 1/2).
// It falls back to the filename stem / "Regular" when the font cannot be
// parsed or carries no usable names.
func familyStyle(path, stem string) (string, string) {
	family, style := stem, "Regular"
	data, err := os.ReadFile(path)
	if err != nil {
		return family, style
	}
	font, err := sfnt.Parse(data)
	if err != nil {
		return family, style
	}
	var buf sfnt.Buffer
	if s, err := font.Name(&buf, sfnt.NameIDTypographicFamily); err == nil && s != "" {
		family = s
	} else if s, err := font.Name(&buf, sfnt.NameIDFamily); err == nil && s != "" {
		family = s
	}
	if s, err := font.Name(&buf, sfnt.NameIDTypographicSubfamily); err == nil && s != "" {
		style = s
	} else if s, err := font.Name(&buf, sfnt.NameIDSubfamily); err == nil && s != "" {
		style = s
	}
	return family, style
}

func isFontFile(name string) bool {
	ext := strings.ToLower(filepath.Ext(name))
	return ext == ".ttf" || ext == ".otf"
}

// ListLibrary lists all .ttf/.otf fonts in the library, sorted by name
// case-insensitively. It returns an empty slice and nil error when the
// library directory does not exist or is empty.
func ListLibrary() ([]FontEntry, error) {
	dir := paths.FontsDir()
	fi, err := os.Stat(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return []FontEntry{}, nil
		}
		return nil, err
	}
	if !fi.IsDir() {
		return []FontEntry{}, nil
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	out := []FontEntry{}
	for _, e := range entries {
		if e.IsDir() || !isFontFile(e.Name()) {
			continue
		}
		info, err := e.Info()
		if err != nil || !info.Mode().IsRegular() {
			continue
		}
		full := filepath.Join(dir, e.Name())
		stem := strings.TrimSuffix(e.Name(), filepath.Ext(e.Name()))
		family, style := familyStyle(full, stem)
		out = append(out, FontEntry{
			Name:   e.Name(),
			Path:   full,
			Size:   info.Size(),
			Mtime:  info.ModTime(),
			Family: family,
			Style:  style,
		})
	}
	sort.SliceStable(out, func(i, j int) bool {
		li, lj := strings.ToLower(out[i].Name), strings.ToLower(out[j].Name)
		if li != lj {
			return li < lj
		}
		return out[i].Name < out[j].Name
	})
	return out, nil
}

// ReadActive returns the active font file per slot. Slots whose file is
// missing are absent from the map.
func ReadActive() (map[string]string, error) {
	active := map[string]string{}
	for slot := range paths.SlotFiles {
		p, err := paths.FontSlotPath(slot)
		if err != nil {
			continue
		}
		fi, err := os.Stat(p)
		if err != nil || !fi.Mode().IsRegular() {
			continue
		}
		active[slot] = p
	}
	return active, nil
}
