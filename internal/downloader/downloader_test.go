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

	if _, err := downloader.Fetch("TestSkip", false, nil); err != nil {
		t.Fatalf("first fetch: %v", err)
	}
	if gets != 1 {
		t.Fatalf("gets after first fetch = %d, want 1", gets)
	}
	if _, err := downloader.Fetch("TestSkip", false, nil); err != nil {
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

	dest, err := downloader.Fetch("TestNoCL", false, nil)
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
	if _, err := downloader.Fetch("Test404", false, nil); err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if gets != 1 {
		t.Fatalf("gets = %d, want 1 (error HEAD must not skip download)", gets)
	}
}

// TestFetch_ReportsProgress pins the measured-progress hook: the body is
// served with a known Content-Length, the callback fires at least twice
// with non-decreasing totals, and the last call lands exactly on
// (len(data), len(data)) — fraction 1.0.
func TestFetch_ReportsProgress(t *testing.T) {
	t.Setenv("TERMUX_HOME", t.TempDir())
	data := fontBytes(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", fmt.Sprint(len(data)))
		w.Write(data)
	}))
	defer srv.Close()
	downloader.NerdFonts["TestProg"] = srv.URL + "/TestProgNerdFont.ttf"
	defer delete(downloader.NerdFonts, "TestProg")

	type call struct{ done, total int64 }
	var calls []call
	if _, err := downloader.Fetch("TestProg", false, func(done, total int64) {
		calls = append(calls, call{done, total})
	}); err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if len(calls) < 2 {
		t.Fatalf("progress hook fired %d time(s), want >= 2", len(calls))
	}
	for i, c := range calls {
		if i > 0 && c.done < calls[i-1].done {
			t.Fatalf("downloaded went backwards at call %d: %d after %d", i, c.done, calls[i-1].done)
		}
		if c.total != int64(len(data)) {
			t.Fatalf("call %d total = %d, want %d (known Content-Length)", i, c.total, len(data))
		}
	}
	last := calls[len(calls)-1]
	if last.done != int64(len(data)) || last.total != int64(len(data)) {
		t.Fatalf("last call = (%d, %d), want (%d, %d) (fraction 1.0)",
			last.done, last.total, len(data), len(data))
	}
}

// TestFetch_UnknownLengthReportsUnknown pins the honest fallback: a
// chunked response has ContentLength == -1, so the hook must report
// exactly one (0, -1) call and never pretend a fraction.
func TestFetch_UnknownLengthReportsUnknown(t *testing.T) {
	t.Setenv("TERMUX_HOME", t.TempDir())
	data := fontBytes(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Two chunks with a Flush in between force chunked encoding,
		// so the client sees ContentLength == -1.
		w.Write(data[:len(data)/2])
		w.(http.Flusher).Flush()
		w.Write(data[len(data)/2:])
	}))
	defer srv.Close()
	downloader.NerdFonts["TestChunked"] = srv.URL + "/TestChunkedNerdFont.ttf"
	defer delete(downloader.NerdFonts, "TestChunked")

	var calls [][2]int64
	dest, err := downloader.Fetch("TestChunked", false, func(done, total int64) {
		calls = append(calls, [2]int64{done, total})
	})
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	got, err := os.ReadFile(dest)
	if err != nil || string(got) != string(data) {
		t.Fatalf("chunked body not stored intact (err=%v)", err)
	}
	if len(calls) != 1 || calls[0][0] != 0 || calls[0][1] != -1 {
		t.Fatalf("callbacks = %v, want exactly one (0, -1)", calls)
	}
}
