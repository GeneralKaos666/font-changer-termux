// Package paths centralizes every filesystem location used by termux-fonts.
//
// All paths funnel through TermuxDir so tests can point the whole
// application at a temporary directory via the TERMUX_HOME env var.
package paths

import (
	"fmt"
	"os"
	"path/filepath"
)

// SlotFiles maps each font slot name to its active file inside TermuxDir.
var SlotFiles = map[string]string{
	"regular":     "font.ttf",
	"bold":        "font-bold.ttf",
	"italic":      "font-italic.ttf",
	"bold-italic": "font-bold-italic.ttf",
}

// SLOT_FILES is an alias of SlotFiles for consumers using the Python-style name.
var SLOT_FILES = SlotFiles

// SlotNames is the canonical slot order — the s-key cycle and the CLI
// help text both derive from it.
var SlotNames = []string{"regular", "bold", "italic", "bold-italic"}

// TermuxDir returns the Termux configuration directory (~/.termux).
// The TERMUX_HOME environment variable wins when set and non-empty,
// otherwise it falls back to $HOME/.termux.
func TermuxDir() string {
	if override := os.Getenv("TERMUX_HOME"); override != "" {
		return override
	}
	home := os.Getenv("HOME")
	if home == "" {
		if h, err := os.UserHomeDir(); err == nil {
			home = h
		}
	}
	return filepath.Join(home, ".termux")
}

// FontsDir returns the directory holding the user's font library
// (~/.termux/fonts).
func FontsDir() string {
	return filepath.Join(TermuxDir(), "fonts")
}

// FontSlotPath returns the active font file for the given slot
// (e.g. "regular" -> ~/.termux/font.ttf). An unknown slot returns an error.
func FontSlotPath(slot string) (string, error) {
	name, ok := SlotFiles[slot]
	if !ok {
		return "", fmt.Errorf("unknown font slot: %q", slot)
	}
	return filepath.Join(TermuxDir(), name), nil
}

// BackupsDir returns the directory where timestamped backups are written
// (~/.termux/backups).
func BackupsDir() string {
	return filepath.Join(TermuxDir(), "backups")
}

// ColorsPath is the Termux colors.properties file.
func ColorsPath() string { return filepath.Join(TermuxDir(), "colors.properties") }
