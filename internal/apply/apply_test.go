package apply

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/GeneralKaos666/font-changer-termux/internal/paths"
)

func useTermuxHome(t *testing.T) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), "termux")
	t.Setenv("TERMUX_HOME", root)
	return root
}

// noReload forces ReloadSettings down the missing-binary path so tests are
// hermetic on machines that do (or do not) ship termux-reload-settings.
func noReload(t *testing.T) {
	t.Helper()
	t.Setenv("PATH", t.TempDir())
}

func fixture(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(p, data, 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

// setupRegular installs fixture a.ttf as the regular slot's original and
// returns (target, originalBytes, state).
func setupRegular(t *testing.T) (string, []byte, *SessionState) {
	t.Helper()
	useTermuxHome(t)
	noReload(t)
	target, err := paths.FontSlotPath("regular")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatal(err)
	}
	orig, err := os.ReadFile(fixture(t, "a.ttf"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, orig, 0o644); err != nil {
		t.Fatal(err)
	}
	return target, orig, NewSessionState()
}

func countBackups(t *testing.T, pattern string) int {
	t.Helper()
	m, err := filepath.Glob(filepath.Join(paths.BackupsDir(), pattern))
	if err != nil {
		t.Fatal(err)
	}
	return len(m)
}

func TestBackupOnce_Reused(t *testing.T) {
	target, _, st := setupRegular(t)
	a := fixture(t, "a.ttf")
	b := fixture(t, "b.ttf")
	if _, err := PreviewFont(a, "regular", st); err != nil {
		t.Fatal(err)
	}
	if _, err := PreviewFont(b, "regular", st); err != nil {
		t.Fatal(err)
	}
	if n := countBackups(t, "font-*.ttf"); n != 1 {
		t.Fatalf("two previews same slot: %d backups, want 1", n)
	}
	if got, _ := os.ReadFile(target); string(got) == "" {
		t.Fatalf("target is empty after preview")
	}
	want, _ := os.ReadFile(b)
	got, _ := os.ReadFile(target)
	if string(got) != string(want) {
		t.Fatalf("last preview did not win")
	}
}

func TestPreview_MultiSlotIndependent(t *testing.T) {
	useTermuxHome(t)
	noReload(t)
	for slot, fam := range map[string]string{"regular": "a.ttf", "bold": "b.ttf"} {
		target, err := paths.FontSlotPath(slot)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			t.Fatal(err)
		}
		data, err := os.ReadFile(fixture(t, fam))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(target, data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	regTarget, _ := paths.FontSlotPath("regular")
	boldTarget, _ := paths.FontSlotPath("bold")
	regOrig, _ := os.ReadFile(regTarget)
	boldOrig, _ := os.ReadFile(boldTarget)
	st := NewSessionState()
	newBold := fixture(t, "a.ttf")
	newReg := fixture(t, "b.ttf")
	if _, err := PreviewFont(newBold, "bold", st); err != nil {
		t.Fatal(err)
	}
	if _, err := PreviewFont(newReg, "regular", st); err != nil {
		t.Fatal(err)
	}
	if n := countBackups(t, "font-*.ttf"); n != 2 {
		t.Fatalf("bold+regular previews: %d backups, want 2", n)
	}
	if n := countBackups(t, "font-bold-*.ttf"); n != 1 {
		t.Fatalf("bold backups: %d, want 1", n)
	}
	m, _ := filepath.Glob(filepath.Join(paths.BackupsDir(), "font-bold-*.ttf"))
	bkBold, _ := os.ReadFile(m[0])
	if string(bkBold) != string(boldOrig) {
		t.Fatalf("bold backup does not hold original bytes")
	}
	m, _ = filepath.Glob(filepath.Join(paths.BackupsDir(), "font-*.ttf"))
	foundReg := false
	for _, p := range m {
		data, _ := os.ReadFile(p)
		if string(data) == string(regOrig) {
			foundReg = true
		}
	}
	if !foundReg {
		t.Fatalf("no regular backup holds original bytes")
	}
	gotReg, _ := os.ReadFile(regTarget)
	wantReg, _ := os.ReadFile(newReg)
	if string(gotReg) != string(wantReg) {
		t.Fatalf("regular target not updated")
	}
}

func TestCommit_ClearsDirty(t *testing.T) {
	_, _, st := setupRegular(t)
	if _, err := PreviewFont(fixture(t, "b.ttf"), "regular", st); err != nil {
		t.Fatal(err)
	}
	if !IsPreviewDirty(st) {
		t.Fatalf("expected dirty after preview")
	}
	if got := CommitPreview(st); got == "" {
		t.Fatalf("CommitPreview returned empty target")
	}
	if IsPreviewDirty(st) {
		t.Fatalf("expected clean after commit")
	}
}

func TestRestore_AfterPreview(t *testing.T) {
	target, original, st := setupRegular(t)
	if _, err := PreviewFont(fixture(t, "b.ttf"), "regular", st); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(target); string(got) == string(original) {
		t.Fatalf("target unchanged after preview")
	}
	ok, err := RestoreOriginal(st)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatalf("RestoreOriginal = false, want true")
	}
	if got, _ := os.ReadFile(target); string(got) != string(original) {
		t.Fatalf("original bytes not restored")
	}
	if IsPreviewDirty(st) {
		t.Fatalf("expected clean after restore")
	}
}

func TestPreview_InvalidRejected(t *testing.T) {
	_, _, st := setupRegular(t)
	bad := filepath.Join(t.TempDir(), "bad.ttf")
	if err := os.WriteFile(bad, []byte("not a font"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := PreviewFont(bad, "regular", st); err == nil {
		t.Fatalf("PreviewFont(bad) expected error, got nil")
	}
	if _, err := InstallFont(bad, "regular"); err == nil {
		t.Fatalf("InstallFont(bad) expected error, got nil")
	}
}

func TestReload_MissingBinaryWarns(t *testing.T) {
	noReload(t)
	if ReloadSettings() {
		t.Fatalf("ReloadSettings with missing binary = true, want false")
	}
	if got := LastReloadOK(); got == nil || *got {
		t.Fatalf("LastReloadOK = %v, want pointer to false", got)
	}
	// File must be kept: install succeeds even though reload fails.
	useTermuxHome(t)
	noReload(t)
	target, err := paths.FontSlotPath("regular")
	if err != nil {
		t.Fatal(err)
	}
	src := fixture(t, "a.ttf")
	got, err := InstallFont(src, "regular")
	if err != nil {
		t.Fatal(err)
	}
	if got != target {
		t.Fatalf("InstallFont = %q, want %q", got, target)
	}
	want, _ := os.ReadFile(src)
	kept, _ := os.ReadFile(target)
	if string(kept) != string(want) {
		t.Fatalf("installed file not kept when reload is missing")
	}
	if ManualRestartHint == "" {
		t.Fatalf("ManualRestartHint is empty")
	}
}

// TestInstallFont_KeepsSymlinkSlot pins write-through semantics: installing
// over a symlinked slot replaces the symlink's target file, and the symlink
// itself must survive. Passes with both direct-write and temp+rename copies.
func TestInstallFont_KeepsSymlinkSlot(t *testing.T) {
	root := useTermuxHome(t)
	noReload(t)
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(root, "real-font.ttf")
	orig, err := os.ReadFile(fixture(t, "a.ttf"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, orig, 0o644); err != nil {
		t.Fatal(err)
	}
	slot, err := paths.FontSlotPath("regular")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(slot), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, slot); err != nil {
		t.Skipf("symlinks unsupported on this filesystem: %v", err)
	}
	src := fixture(t, "b.ttf")
	if _, err := InstallFont(src, "regular"); err != nil {
		t.Fatal(err)
	}
	if fi, err := os.Lstat(slot); err != nil || fi.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("slot is no longer a symlink after install: %v %v", fi, err)
	}
	got, _ := os.ReadFile(target) // writes must land in the real file, through the symlink
	want, _ := os.ReadFile(src)
	if string(got) != string(want) {
		t.Fatal("slot target was not replaced by the installed font")
	}
}

func TestEnsureBackupOnce_MissingTarget(t *testing.T) {
	useTermuxHome(t)
	target, err := paths.FontSlotPath("regular")
	if err != nil {
		t.Fatal(err)
	}
	got, err := EnsureBackupOnce(target)
	if err != nil {
		t.Fatal(err)
	}
	if got != "" {
		t.Fatalf("EnsureBackupOnce(missing) = %q, want empty", got)
	}
}
