package theme

import (
	"os"
	"path/filepath"
	"testing"
)

func writeProps(t *testing.T, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "colors.properties")
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestLoad_FullPalette(t *testing.T) {
	p := writeProps(t, `
# comment
background=#1e1e2e
foreground=#cdd6f4
cursor=#f5e0dc
color0=#11111b
color4=#89b4fa
color8=#585b70
color12=#89b4fa
`)
	pal, err := LoadFile(p)
	if err != nil {
		t.Fatalf("LoadFile: %v", err)
	}
	if pal.Background != "#1e1e2e" || pal.Foreground != "#cdd6f4" {
		t.Fatalf("bg/fg = %q/%q", pal.Background, pal.Foreground)
	}
	if pal.Accent != "#89b4fa" {
		t.Fatalf("accent = %q, want color12", pal.Accent)
	}
	if pal.Muted != "#585b70" {
		t.Fatalf("muted = %q, want color8", pal.Muted)
	}
}

func TestLoad_Fallbacks(t *testing.T) {
	// color12 missing → color4; color8 missing → "".
	p := writeProps(t, "color4=#ff0000\n")
	pal, err := LoadFile(p)
	if err != nil {
		t.Fatalf("LoadFile: %v", err)
	}
	if pal.Accent != "#ff0000" {
		t.Fatalf("accent = %q, want color4 fallback", pal.Accent)
	}
	if pal.Muted != "" {
		t.Fatalf("muted = %q, want empty", pal.Muted)
	}
}

func TestLoad_MissingFile(t *testing.T) {
	if _, err := LoadFile(filepath.Join(t.TempDir(), "nope.properties")); err == nil {
		t.Fatal("expected error for missing file, got nil")
	}
}

func TestLoad_IgnoresGarbage(t *testing.T) {
	p := writeProps(t, "not-a-color-line\ncolor3=red\ncolor5=#12345\ncolor6=#aabbcc\n")
	pal, err := LoadFile(p)
	if err != nil {
		t.Fatalf("LoadFile: %v", err)
	}
	if pal.Accent != "" {
		t.Fatalf("accent = %q, want empty (no color4/12)", pal.Accent)
	}
}
