package tui

import (
	"os"
	"path/filepath"
	"testing"
)

func TestExpandHome(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	if got, want := expandHome("~/x.ttf"), filepath.Join(home, "x.ttf"); got != want {
		t.Fatalf("expandHome(%q) = %q, want %q", "~/x.ttf", got, want)
	}
	if got, want := expandHome("~"), home; got != want {
		t.Fatalf("expandHome(~) = %q, want %q", got, want)
	}
	if got := expandHome("/abs/x.ttf"); got != "/abs/x.ttf" {
		t.Fatalf("absolute path changed: %q", got)
	}
	if got := expandHome("rel/x.ttf"); got != "rel/x.ttf" {
		t.Fatalf("relative path changed: %q", got)
	}
	if got := expandHome("~user/x.ttf"); got != "~user/x.ttf" {
		t.Fatalf("non-local ~ path changed: %q", got)
	}
}

func TestCompletePath_SingleMatchCompletes(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "Hack.ttf"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	in := filepath.Join(dir, "Ha")
	want := filepath.Join(dir, "Hack.ttf")
	if got := completePath(in); got != want {
		t.Fatalf("completePath(%q) = %q, want %q", in, got, want)
	}
}

func TestCompletePath_MultipleMatchesCompleteToCommonPrefix(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"AB.ttf", "ABC.ttf"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	in := filepath.Join(dir, "a")
	want := filepath.Join(dir, "AB")
	if got := completePath(in); got != want {
		t.Fatalf("completePath(%q) = %q, want %q", in, got, want)
	}
}

func TestCompletePath_NoMatchLeavesInputUnchanged(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "Hack.ttf"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	in := filepath.Join(dir, "zz")
	if got := completePath(in); got != in {
		t.Fatalf("completePath(%q) = %q, want it unchanged", in, got)
	}
}

func TestCompletePath_DirectoryGetsTrailingSlash(t *testing.T) {
	dir := t.TempDir()
	if got := completePath(dir); got != dir+string(filepath.Separator) {
		t.Fatalf("completePath(%q) = %q, want a trailing separator", dir, got)
	}
	// An already-suffixed directory path is left alone.
	if got := completePath(dir + string(filepath.Separator)); got != dir+string(filepath.Separator) {
		t.Fatalf("completePath of a slash-terminated dir = %q, want it unchanged", got)
	}
}

func TestCompletePath_BareRelativeNameIsNoOp(t *testing.T) {
	if got := completePath("Hack"); got != "Hack" {
		t.Fatalf("completePath(Hack) = %q, want it unchanged (no directory part)", got)
	}
}

func TestCompletePath_ExpandsHomeFirst(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	if err := os.WriteFile(filepath.Join(home, "import.ttf"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(home, "import.ttf")
	if got := completePath("~/import.ttf"); got != want {
		t.Fatalf("completePath(~/import.ttf) = %q, want %q", got, want)
	}
}
