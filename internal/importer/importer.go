// Package importer copies external font files into the font library
// (~/.termux/fonts) after validation.
package importer

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"termux-fonts-go/internal/paths"
	"termux-fonts-go/internal/validate"
)

// storageHint explains the shared-storage permission fix, mirroring the
// Python implementation.
const storageHint = "permission denied: if the file lives on shared storage " +
	"(/sdcard/Download/...), run `termux-setup-storage`, grant the storage " +
	"permission, and retry"

func storageError(where string, err error) error {
	return fmt.Errorf("%s (%s: %v)", storageHint, where, err)
}

func wrapOSError(where string, err error) error {
	if err == nil {
		return nil
	}
	if os.IsPermission(err) {
		return storageError(where, err)
	}
	return err
}

// ResolveClash returns the first free "<stem>-N<suffix>" sibling of dest.
func ResolveClash(dest string) string {
	ext := filepath.Ext(dest)
	stem := strings.TrimSuffix(filepath.Base(dest), ext)
	dir := filepath.Dir(dest)
	for i := 1; ; i++ {
		candidate := filepath.Join(dir, fmt.Sprintf("%s-%d%s", stem, i, ext))
		if _, err := os.Stat(candidate); errors.Is(err, os.ErrNotExist) {
			return candidate
		}
	}
}

// ImportFile validates src and copies it into the font library.
//
// clash controls name collisions: "error" raises FileExistsError,
// "keep-both" picks <name>-N.ttf, "replace" overwrites.
func ImportFile(src, clash string) (string, error) {
	switch clash {
	case "error", "keep-both", "replace":
	default:
		return "", fmt.Errorf("unknown clash mode %q; choose from error, keep-both, replace", clash)
	}
	ok, reason := validate.IsValidFont(src)
	if !ok {
		lowered := strings.ToLower(reason)
		if strings.Contains(lowered, "permission denied") || strings.Contains(lowered, "errno 13") {
			return "", storageError(src, errors.New(reason))
		}
		return "", fmt.Errorf("invalid font %s: %s", src, reason)
	}
	destDir := paths.FontsDir()
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return "", wrapOSError(destDir, err)
	}
	dest := filepath.Join(destDir, filepath.Base(src))
	if sameFile(src, dest) {
		return dest, nil
	}
	if _, err := os.Stat(dest); err == nil {
		switch clash {
		case "error":
			return "", fmt.Errorf("font already in library: %s", filepath.Base(dest))
		case "keep-both":
			dest = ResolveClash(dest)
		}
	}
	if err := copyFile(src, dest); err != nil {
		return "", wrapOSError(src, err)
	}
	return dest, nil
}

func sameFile(a, b string) bool {
	ra, errA := filepath.EvalSymlinks(a)
	rb, errB := filepath.EvalSymlinks(b)
	if errA != nil || errB != nil {
		return false
	}
	return ra == rb
}

func copyFile(src, dest string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dest, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}
