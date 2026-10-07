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
// Stat errors other than "not exist" (e.g. permission denied) are
// returned instead of looping forever.
func ResolveClash(dest string) (string, error) {
	ext := filepath.Ext(dest)
	stem := strings.TrimSuffix(filepath.Base(dest), ext)
	dir := filepath.Dir(dest)
	for i := 1; ; i++ {
		candidate := filepath.Join(dir, fmt.Sprintf("%s-%d%s", stem, i, ext))
		if _, err := os.Stat(candidate); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return candidate, nil
			}
			return "", err
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
			var err error
			dest, err = ResolveClash(dest)
			if err != nil {
				return "", wrapOSError(filepath.Dir(dest), err)
			}
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
	// Write to a temp sibling and rename, so an interrupted copy never
	// leaves a truncated dest (mirrors the downloader).
	out, err := os.CreateTemp(filepath.Dir(dest), ".import-*.part")
	if err != nil {
		return err
	}
	tmp := out.Name()
	defer os.Remove(tmp)
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	if err := out.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmp, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, dest)
}
