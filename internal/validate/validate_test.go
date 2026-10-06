package validate

import (
	"os"
	"path/filepath"
	"testing"
)

func writeTemp(t *testing.T, name string, data []byte) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(p, data, 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestIsValidFont_RejectsGarbage(t *testing.T) {
	p := writeTemp(t, "bad.ttf", []byte("not a font"))
	ok, reason := IsValidFont(p)
	if ok {
		t.Fatalf("expected (false, reason) for garbage file, got valid")
	}
	if reason == "" {
		t.Fatalf("expected non-empty reason for garbage file")
	}
}

func TestIsValidFont_RejectsCorruptTables(t *testing.T) {
	// Review Focus: valid magic but broken tables must be rejected.
	data := append([]byte{0x00, 0x01, 0x00, 0x00}, make([]byte, 100)...)
	p := writeTemp(t, "corrupt.ttf", data)
	ok, reason := IsValidFont(p)
	if ok {
		t.Fatalf("expected (false, reason) for corrupt tables, got valid")
	}
	if reason == "" {
		t.Fatalf("expected non-empty reason for corrupt tables")
	}
}
