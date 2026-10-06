package scan

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"termux-fonts-go/internal/paths"
)

func useTermuxHome(t *testing.T) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), "termux")
	t.Setenv("TERMUX_HOME", root)
	return root
}

func writeFont(t *testing.T, dir, name string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), make([]byte, 16), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestListLibrary_Sorted(t *testing.T) {
	useTermuxHome(t)
	if err := os.MkdirAll(paths.FontsDir(), 0o755); err != nil {
		t.Fatal(err)
	}
	names := []string{"Zebra.ttf", "apple.ttf", "Mango.OTF"}
	for _, n := range names {
		writeFont(t, paths.FontsDir(), n)
	}
	entries, err := ListLibrary()
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != len(names) {
		t.Fatalf("expected %d entries, got %d", len(names), len(entries))
	}
	var got []string
	for _, e := range entries {
		got = append(got, e.Name)
		if e.Family == "" || e.Style == "" {
			t.Fatalf("entry %q missing family/style fallback", e.Name)
		}
		if e.Size != 16 {
			t.Fatalf("entry %q size = %d, want 16", e.Name, e.Size)
		}
	}
	for i := 1; i < len(got); i++ {
		if strings.ToLower(got[i-1]) > strings.ToLower(got[i]) {
			t.Fatalf("not sorted case-insensitively: %v", got)
		}
	}
}

func TestReadActive_MissingSlot(t *testing.T) {
	useTermuxHome(t)
	active, err := ReadActive()
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := active["bold"]; ok {
		t.Fatalf("expected bold slot absent, got %q", active["bold"])
	}
}

func TestListLibrary_Empty(t *testing.T) {
	// Review Focus: missing fonts dir -> empty, nil error (no crash).
	useTermuxHome(t)
	entries, err := ListLibrary()
	if err != nil {
		t.Fatalf("expected nil error for missing dir, got %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("expected empty library, got %v", entries)
	}
}
