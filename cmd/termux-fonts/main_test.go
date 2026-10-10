package main

import (
	"os"
	"path/filepath"
	"testing"

	"termux-fonts-go/internal/scan"
)

func entries(names ...string) []scan.FontEntry {
	out := make([]scan.FontEntry, 0, len(names))
	for _, n := range names {
		out = append(out, scan.FontEntry{Name: n, Path: filepath.Join("/fonts", n)})
	}
	return out
}

func TestResolveMatch_ExactWins(t *testing.T) {
	got, ambiguous, found := resolveMatch(entries("a.ttf", "A.TTF"), "A.TTF")
	if !found || ambiguous {
		t.Fatalf("resolveMatch exact = found=%v ambiguous=%v, want true,false", found, ambiguous)
	}
	if got.Name != "A.TTF" {
		t.Fatalf("resolveMatch exact = %q, want %q", got.Name, "A.TTF")
	}
}

func TestResolveMatch_Casefold(t *testing.T) {
	got, ambiguous, found := resolveMatch(entries("JetBrainsMono.ttf"), "jetbrainsmono.TTF")
	if !found || ambiguous {
		t.Fatalf("resolveMatch casefold = found=%v ambiguous=%v, want true,false", found, ambiguous)
	}
	if got.Name != "JetBrainsMono.ttf" {
		t.Fatalf("resolveMatch casefold = %q, want JetBrainsMono.ttf", got.Name)
	}
}

func TestResolveMatch_Ambiguous(t *testing.T) {
	// No exact hit for "b.ttf"; two casefold hits.
	_, ambiguous, found := resolveMatch(entries("B.ttf", "b.TTF"), "b.ttf")
	if !found || !ambiguous {
		t.Fatalf("resolveMatch ambiguous = found=%v ambiguous=%v, want true,true", found, ambiguous)
	}
}

func TestResolveMatch_Unknown(t *testing.T) {
	_, ambiguous, found := resolveMatch(entries("a.ttf"), "nope.ttf")
	if found || ambiguous {
		t.Fatalf("resolveMatch unknown = found=%v ambiguous=%v, want false,false", found, ambiguous)
	}
}

func TestCasefoldHits(t *testing.T) {
	got := casefoldHits(entries("B.ttf", "b.TTF", "A.ttf"), "b.ttf")
	want := []string{"B.ttf", "b.TTF"}
	if len(got) != len(want) {
		t.Fatalf("casefoldHits = %v, want %v", got, want)
	}
	for i := range want {
		if got[i].Name != want[i] {
			t.Fatalf("casefoldHits[%d] = %q, want %q", i, got[i].Name, want[i])
		}
	}
}

func TestSeed_EmptyLibrary(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("TERMUX_HOME", dir)
	// Active regular slot file present, library empty.
	if err := os.WriteFile(filepath.Join(dir, "font.ttf"), []byte("fakefont"), 0o644); err != nil {
		t.Fatal(err)
	}
	got := ensureBuiltinSeed()
	if got == "" {
		t.Fatal("ensureBuiltinSeed = empty, want fonts/Current.ttf")
	}
	if _, err := os.Stat(filepath.Join(dir, "fonts", "Current.ttf")); err != nil {
		t.Fatalf("seeded file missing: %v", err)
	}
}

func TestSeed_NoSeedWhenLibraryNonEmpty(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("TERMUX_HOME", dir)
	if err := os.MkdirAll(filepath.Join(dir, "fonts"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "fonts", "Hack.ttf"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := ensureBuiltinSeed(); got != "" {
		t.Fatalf("ensureBuiltinSeed = %q, want empty (library non-empty)", got)
	}
}

func TestRealMain_ApplyEmptyIsUnknown(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("TERMUX_HOME", dir)
	if got := realMain([]string{"--apply", ""}); got != 2 {
		t.Fatalf("realMain(--apply \"\") = %d, want 2 (unknown font, matching Python)", got)
	}
}

func TestRealMain_ApplyUnknownIsExit2(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("TERMUX_HOME", dir)
	if got := realMain([]string{"--apply", "nope.ttf"}); got != 2 {
		t.Fatalf("realMain(--apply nope.ttf) = %d, want 2", got)
	}
}

func TestRealMain_UnknownSlotIsExit2(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("TERMUX_HOME", dir)
	if got := realMain([]string{"--apply", "Hack.ttf", "--slot", "bogus"}); got != 2 {
		t.Fatalf("realMain(--apply Hack.ttf --slot bogus) = %d, want 2", got)
	}
}
