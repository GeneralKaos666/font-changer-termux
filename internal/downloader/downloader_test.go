package downloader_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"termux-fonts-go/internal/downloader"
	"termux-fonts-go/internal/paths"
)

// TestNerdFonts_AllFamiliesRegular pins the expanded catalog: one healthy
// Regular weight per Nerd Fonts family, all on the pinned release base.
func TestNerdFonts_AllFamiliesRegular(t *testing.T) {
	const prefix = "https://github.com/ryanoasis/nerd-fonts/raw/v3.2.1/patched-fonts/"
	if len(downloader.NerdFonts) < 46 {
		t.Fatalf("catalog has %d entries, want at least 46 (one per family)", len(downloader.NerdFonts))
	}
	for name, url := range downloader.NerdFonts {
		if !strings.HasPrefix(url, prefix) {
			t.Errorf("%s: url %q is not on the pinned v3.2.1 base", name, url)
		}
		if !strings.HasSuffix(url, ".ttf") {
			t.Errorf("%s: url %q does not point at a .ttf", name, url)
		}
		if strings.Contains(name, "Propo") {
			t.Errorf("%s: proportional variant should not be in the terminal catalog", name)
		}
	}
	for _, want := range []string{
		"Hack-Regular", "FiraCode-Regular", "JetBrainsMono-Regular",
		"IosevkaTerm-Regular", "CascadiaCode-Regular", "SourceCodePro-Regular",
		"IBMPlexMono-Regular", "Mononoki-Regular", "Terminus-Regular",
		"AnonymousPro-Regular", "NotoSansMono-Regular", "VictorMono-Regular",
		"UbuntuMono-Regular", "ProggyClean-Regular", "iAWriterMono-Regular",
	} {
		if _, ok := downloader.NerdFonts[want]; !ok {
			t.Errorf("catalog is missing family %s", want)
		}
	}
}

func fontBytes(t *testing.T) []byte {
	t.Helper()
	data, err := os.ReadFile("../apply/testdata/a.ttf")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	return data
}

func TestFetch_SkipsIfSameSize(t *testing.T) {
	t.Setenv("TERMUX_HOME", t.TempDir())
	data := fontBytes(t)
	var gets int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodHead {
			w.Header().Set("Content-Length", fmt.Sprint(len(data)))
			return
		}
		gets++
		w.Write(data)
	}))
	defer srv.Close()
	downloader.NerdFonts["TestSkip"] = srv.URL + "/TestSkipNerdFont.ttf"
	defer delete(downloader.NerdFonts, "TestSkip")

	if _, err := downloader.Fetch("TestSkip", false); err != nil {
		t.Fatalf("first fetch: %v", err)
	}
	if gets != 1 {
		t.Fatalf("gets after first fetch = %d, want 1", gets)
	}
	if _, err := downloader.Fetch("TestSkip", false); err != nil {
		t.Fatalf("second fetch: %v", err)
	}
	if gets != 1 {
		t.Fatalf("gets after second fetch = %d, want no new download", gets)
	}
}

func TestFetch_NoContentLengthDownloads(t *testing.T) {
	t.Setenv("TERMUX_HOME", t.TempDir())
	data := fontBytes(t)
	// Pre-create a stale dest so the fetch must decide via HEAD alone.
	home := os.Getenv("TERMUX_HOME")
	if err := os.MkdirAll(filepath.Join(home, "fonts"), 0o755); err != nil {
		t.Fatal(err)
	}
	stale := filepath.Join(home, "fonts", "NoCLNerdFont.ttf")
	if err := os.WriteFile(stale, []byte("stale"), 0o644); err != nil {
		t.Fatal(err)
	}
	var gets int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodHead {
			// Deliberately omit Content-Length: the fetch must
			// fall back to downloading.
			w.WriteHeader(http.StatusOK)
			return
		}
		gets++
		w.Write(data)
	}))
	defer srv.Close()
	downloader.NerdFonts["TestNoCL"] = srv.URL + "/NoCLNerdFont.ttf"
	defer delete(downloader.NerdFonts, "TestNoCL")

	dest, err := downloader.Fetch("TestNoCL", false)
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if gets != 1 {
		t.Fatalf("gets = %d, want 1 (fallback download)", gets)
	}
	got, _ := os.ReadFile(dest)
	if string(got) != string(data) {
		t.Fatal("dest was not replaced by the download")
	}
}

func TestFetch_HeadErrorStatusDownloads(t *testing.T) {
	t.Setenv("TERMUX_HOME", t.TempDir())
	data := fontBytes(t)
	var gets int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodHead {
			// 404 with a Content-Length matching local size must NOT skip.
			w.Header().Set("Content-Length", fmt.Sprint(len(data)))
			w.WriteHeader(http.StatusNotFound)
			return
		}
		gets++
		w.Write(data)
	}))
	defer srv.Close()
	downloader.NerdFonts["Test404"] = srv.URL + "/Test404NerdFont.ttf"
	defer delete(downloader.NerdFonts, "Test404")

	// Pre-seed a same-size file so a naive size comparison would skip.
	dest := filepath.Join(paths.FontsDir(), "Test404NerdFont.ttf")
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dest, make([]byte, len(data)), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := downloader.Fetch("Test404", false); err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if gets != 1 {
		t.Fatalf("gets = %d, want 1 (error HEAD must not skip download)", gets)
	}
}
