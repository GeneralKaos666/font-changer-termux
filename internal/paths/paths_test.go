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

func TestSlotNamesMatchesSlotFilesInCycleOrder(t *testing.T) {
	want := []string{"regular", "bold", "italic", "bold-italic"}
	if len(SlotNames) != len(want) {
		t.Fatalf("SlotNames = %v, want %v", SlotNames, want)
	}
	for i, name := range want {
		if SlotNames[i] != name {
			t.Fatalf("SlotNames[%d] = %q, want %q (cycle order, not alphabetical)", i, SlotNames[i], name)
		}
	}
	// SlotNames must cover exactly the SlotFiles keys — no more, no less.
	for _, name := range SlotNames {
		if _, ok := SlotFiles[name]; !ok {
			t.Fatalf("SlotNames lists %q, missing from SlotFiles", name)
		}
	}
	for name := range SlotFiles {
		found := false
		for _, n := range SlotNames {
			if n == name {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("SlotFiles key %q missing from SlotNames", name)
		}
	}
}

func TestColorsPath(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("TERMUX_HOME", dir)
	if got := ColorsPath(); got != filepath.Join(dir, "colors.properties") {
		t.Fatalf("ColorsPath() = %q, want %q", got, filepath.Join(dir, "colors.properties"))
	}
}
