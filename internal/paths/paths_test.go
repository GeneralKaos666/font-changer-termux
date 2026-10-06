package paths

import (
	"path/filepath"
	"testing"
)

func TestTermuxHomeOverride(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("TERMUX_HOME", dir)
	if got := FontsDir(); got != filepath.Join(dir, "fonts") {
		t.Fatalf("FontsDir() = %q, want %q", got, filepath.Join(dir, "fonts"))
	}
	if got, _ := FontSlotPath("regular"); got != filepath.Join(dir, "font.ttf") {
		t.Fatalf("FontSlotPath(regular) = %q, want %q", got, filepath.Join(dir, "font.ttf"))
	}
	if got, _ := FontSlotPath("bold"); got != filepath.Join(dir, "font-bold.ttf") {
		t.Fatalf("FontSlotPath(bold) = %q, want %q", got, filepath.Join(dir, "font-bold.ttf"))
	}
}

func TestTermuxDirOverrideAndBackups(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("TERMUX_HOME", dir)
	if got := TermuxDir(); got != dir {
		t.Fatalf("TermuxDir() = %q, want %q", got, dir)
	}
	if got := BackupsDir(); got != filepath.Join(dir, "backups") {
		t.Fatalf("BackupsDir() = %q, want %q", got, filepath.Join(dir, "backups"))
	}
}

func TestFontSlotPathUnknownSlot(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("TERMUX_HOME", dir)
	if _, err := FontSlotPath("nope"); err == nil {
		t.Fatal("FontSlotPath(nope) expected error, got nil")
	}
}
