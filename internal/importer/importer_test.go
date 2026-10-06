package importer_test

import (
	"os"
	"path/filepath"
	"testing"

	"termux-fonts-go/internal/importer"
)

const fixture = "../apply/testdata/a.ttf"

func stage(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile(fixture)
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	src := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(src, data, 0o644); err != nil {
		t.Fatalf("stage src: %v", err)
	}
	return src
}

func TestImport_CopiesValid(t *testing.T) {
	t.Setenv("TERMUX_HOME", t.TempDir())
	src := stage(t, "Hack.ttf")
	dest, err := importer.ImportFile(src, "error")
	if err != nil {
		t.Fatalf("ImportFile: %v", err)
	}
	if filepath.Base(dest) != "Hack.ttf" {
		t.Fatalf("dest = %q, want Hack.ttf", dest)
	}
	want, _ := os.ReadFile(src)
	got, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("read dest: %v", err)
	}
	if string(got) != string(want) {
		t.Fatal("dest content differs from src")
	}
}

func TestImport_ClashKeepBoth(t *testing.T) {
	t.Setenv("TERMUX_HOME", t.TempDir())
	if _, err := importer.ImportFile(stage(t, "Hack.ttf"), "error"); err != nil {
		t.Fatalf("first import: %v", err)
	}
	dest, err := importer.ImportFile(stage(t, "Hack.ttf"), "keep-both")
	if err != nil {
		t.Fatalf("second import: %v", err)
	}
	if filepath.Base(dest) != "Hack-1.ttf" {
		t.Fatalf("dest = %q, want Hack-1.ttf", dest)
	}
}

func TestImport_InvalidRejected(t *testing.T) {
	t.Setenv("TERMUX_HOME", t.TempDir())
	src := filepath.Join(t.TempDir(), "junk.ttf")
	if err := os.WriteFile(src, []byte("not a font"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := importer.ImportFile(src, "error"); err == nil {
		t.Fatal("expected error for invalid font, got nil")
	}
}
