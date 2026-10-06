// Package validate checks whether a file is a readable sfnt font.
//
// A file is accepted when its 4-byte sfnt signature matches a known magic
// value and the font parses with a usable name table. Checking the magic
// alone is not enough: a corrupt file can carry a valid signature while
// having broken tables, so a name-table read is forced.
package validate

import (
	"errors"
	"fmt"
	"io"
	"os"

	"golang.org/x/image/font/sfnt"
)

// ValidMagic lists the accepted 4-byte sfnt signatures:
// TrueType/OT-Truetype, OT-CFF, legacy Mac TrueType, PostScript Type 1 wrapper.
var ValidMagic = [][]byte{
	{0x00, 0x01, 0x00, 0x00},
	{'O', 'T', 'T', 'O'},
	{'t', 'r', 'u', 'e'},
	{'t', 'y', 'p', '1'},
}

// IsValidFont returns (true, "") when path is a readable font,
// otherwise (false, reason).
func IsValidFont(path string) (bool, string) {
	fi, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, "file does not exist"
		}
		return false, fmt.Sprintf("cannot read file: %v", err)
	}
	if !fi.Mode().IsRegular() {
		return false, "not a regular file"
	}
	header := make([]byte, 4)
	func() {
		f, err := os.Open(path)
		if err != nil {
			return
		}
		defer f.Close()
		_, err = io.ReadFull(f, header)
		if err != nil {
			header = header[:0]
		}
	}()
	if len(header) < 4 {
		return false, "file is too small to be a font"
	}
	known := false
	for _, magic := range ValidMagic {
		if string(header) == string(magic) {
			known = true
			break
		}
	}
	if !known {
		return false, fmt.Sprintf("unknown font signature: %q", header)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return false, fmt.Sprintf("cannot read file: %v", err)
	}
	font, err := sfnt.Parse(data)
	if err != nil {
		return false, fmt.Sprintf("cannot parse font: %v", err)
	}
	// Force a name-table read; Parse alone may succeed on broken tables.
	// ErrNotFound only means that one record is absent, the table itself
	// parsed, so the font is still valid.
	var buf sfnt.Buffer
	if _, err := font.Name(&buf, sfnt.NameIDFamily); err != nil &&
		!errors.Is(err, sfnt.ErrNotFound) {
		return false, fmt.Sprintf("cannot parse font: %v", err)
	}
	return true, ""
}
