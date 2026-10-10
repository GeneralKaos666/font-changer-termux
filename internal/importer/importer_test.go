package importer_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"termux-fonts-go/internal/importer"
	"termux-fonts-go/internal/paths"
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

func TestImport_AskClashReturnsTypedError(t *testing.T) {
	t.Setenv("TERMUX_HOME", t.TempDir())
	dir := paths.FontsDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(dir, "Name.ttf")
	sentinel := []byte("pre-existing library copy")
	if err := os.WriteFile(dest, sentinel, 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := importer.ImportFile(stage(t, "Name.ttf"), "ask")
	var ce *importer.ClashError
	if !errors.As(err, &ce) {
		t.Fatalf("ImportFile(ask) err = %v, want *ClashError", err)
	}
	if ce.Dest != dest {
		t.Fatalf("ClashError.Dest = %q, want %q", ce.Dest, dest)
	}
	if !strings.Contains(ce.Error(), "Name.ttf") {
		t.Fatalf("ClashError.Error() = %q, want it to name the file", ce.Error())
	}
	got, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("read library file: %v", err)
	}
	if string(got) != string(sentinel) {
		t.Fatal("ask mode modified the existing library file")
	}
}

func TestImport_AskNoClashImports(t *testing.T) {
	t.Setenv("TERMUX_HOME", t.TempDir())
	src := stage(t, "Fresh.ttf")
	dest, err := importer.ImportFile(src, "ask")
	if err != nil {
		t.Fatalf("ImportFile(ask) with no clash: %v", err)
	}
	if filepath.Base(dest) != "Fresh.ttf" {
		t.Fatalf("dest = %q, want Fresh.ttf", dest)
	}
	want, _ := os.ReadFile(src)
	got, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("library did not gain the file: %v", err)
	}
	if string(got) != string(want) {
		t.Fatal("imported content differs from src")
	}
}

func TestImport_AskSameFileIsNotAClash(t *testing.T) {
	t.Setenv("TERMUX_HOME", t.TempDir())
	first, err := importer.ImportFile(stage(t, "Same.ttf"), "error")
	if err != nil {
		t.Fatalf("first import: %v", err)
	}
	dest, err := importer.ImportFile(first, "ask")
	if err != nil {
		t.Fatalf("ImportFile(ask) with src == library file: %v", err)
	}
	if dest != first {
		t.Fatalf("dest = %q, want the same file %q", dest, first)
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

func TestResolveClash_StatErrorPropagated(t *testing.T) {
	t.Setenv("TERMUX_HOME", t.TempDir())
	dir := paths.FontsDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(dir, 0o000); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(dir, 0o755)
	done := make(chan struct{})
	var dest string
	var err error
	go func() {
		defer close(done)
		dest, err = importer.ResolveClash(filepath.Join(dir, "Hack.ttf"))
	}()
	select {
	case <-done:
		if err == nil {
			t.Fatalf("expected stat error, got dest %s", dest)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("ResolveClash hung on stat error (infinite loop)")
	}
}

func TestImport_FailedCopyLeavesNoTemp(t *testing.T) {
	t.Setenv("TERMUX_HOME", t.TempDir())
	dir := paths.FontsDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	src := stage(t, "Src.ttf")
	if err := os.Chmod(dir, 0o555); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(dir, 0o755)
	if _, err := importer.ImportFile(src, "replace"); err == nil {
		t.Fatal("expected copy error in read-only dir")
	}
	leftovers, _ := filepath.Glob(filepath.Join(dir, ".import-*.part"))
	if len(leftovers) != 0 {
		t.Fatalf("temp leftovers: %v", leftovers)
	}
}
