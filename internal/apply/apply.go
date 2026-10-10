// Package apply implements backup-once preview/commit/restore and the
// Termux settings reload used by nerdfont-changer.
package apply

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"time"

	"github.com/GeneralKaos666/nerdfont-changer/internal/paths"
	"github.com/GeneralKaos666/nerdfont-changer/internal/validate"
)

// PreviewRef identifies the font file previewed into a slot.
type PreviewRef struct {
	Src  string
	Slot string
}

// SessionState tracks one preview session: the slot contents seen at
// session start (nil entries for empty slots), which slots have been
// backed up, whether a preview is awaiting commit/restore, and the
// current preview reference.
type SessionState struct {
	Originals map[string][]byte
	BackedUp  map[string]bool
	Dirty     bool
	Preview   *PreviewRef
}

// ManualRestartHint is shown when a font was installed but Termux could
// not be asked to reload (spec manual step: restart the Termux app).
const ManualRestartHint = "termux-reload-settings not available; " +
	"restart the Termux app to apply the font"

var lastReloadOK *bool

func snapshotOriginals() map[string][]byte {
	originals := map[string][]byte{}
	for slot := range paths.SlotFiles {
		target, err := paths.FontSlotPath(slot)
		if err != nil {
			continue
		}
		data, err := os.ReadFile(target)
		if err != nil {
			originals[slot] = nil
			continue
		}
		originals[slot] = data
	}
	return originals
}

// NewSessionState creates fresh preview-session state.
func NewSessionState() *SessionState {
	return &SessionState{
		Originals: snapshotOriginals(),
		BackedUp:  map[string]bool{},
		Dirty:     false,
		Preview:   nil,
	}
}

// isSlotBackup reports whether candidate is a timestamped backup of the
// slot file target: {stem}-YYYY-MM-DD-HHMMSS{suffix} (full match).
func isSlotBackup(candidate, stem, suffix string) bool {
	re := regexp.MustCompile("^" + regexp.QuoteMeta(stem) +
		`-\d{4}-\d{2}-\d{2}-\d{6}` + regexp.QuoteMeta(suffix) + "$")
	return re.MatchString(candidate)
}

// EnsureBackupOnce backs up target once and reuses the existing backup on
// repeat calls. It returns "" with nil error when target does not exist.
func EnsureBackupOnce(target string) (string, error) {
	fi, err := os.Stat(target)
	if err != nil || !fi.Mode().IsRegular() {
		return "", nil
	}
	destDir := paths.BackupsDir()
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return "", err
	}
	base := filepath.Base(target)
	ext := filepath.Ext(base)
	stem := base[:len(base)-len(ext)]
	matches, err := filepath.Glob(filepath.Join(destDir, stem+"-[0-9][0-9][0-9][0-9]-*"+ext))
	if err != nil {
		return "", err
	}
	existing := []string{}
	for _, m := range matches {
		if isSlotBackup(filepath.Base(m), stem, ext) {
			existing = append(existing, m)
		}
	}
	sort.Strings(existing)
	if len(existing) > 0 {
		return existing[0], nil
	}
	timestamp := time.Now().Format("2006-01-02-150405")
	dest := filepath.Join(destDir, stem+"-"+timestamp+ext)
	if _, err := os.Stat(dest); err == nil {
		return dest, nil
	}
	if err := copyPreservingMode(target, dest); err != nil {
		return "", err
	}
	return dest, nil
}

// copyPreservingMode copies src to dst atomically (temp sibling + rename)
// and keeps the source file mode, so an interrupted copy never leaves a
// truncated destination. When dst is a symlink, the symlink's target is
// what gets replaced; the symlink itself survives.
func copyPreservingMode(src, dst string) error {
	resolved := dst
	if r, err := filepath.EvalSymlinks(dst); err == nil {
		resolved = r
	}
	fi, err := os.Stat(src)
	if err != nil {
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	tmp, err := os.CreateTemp(filepath.Dir(resolved), ".apply-*.part")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name()) // no-op once the rename below succeeds
	if _, err := io.Copy(tmp, in); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmp.Name(), fi.Mode().Perm()); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), resolved)
}

func checkedTarget(src, slot string) (string, error) {
	ok, reason := validate.IsValidFont(src)
	if !ok {
		return "", fmt.Errorf("invalid font: %s", reason)
	}
	return paths.FontSlotPath(slot)
}

// InstallFont validates src, backs up the slot once, installs, and reloads.
func InstallFont(src, slot string) (string, error) {
	target, err := checkedTarget(src, slot)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return "", err
	}
	if _, err := EnsureBackupOnce(target); err != nil {
		return "", err
	}
	if err := copyPreservingMode(src, target); err != nil {
		return "", err
	}
	ReloadSettings()
	return target, nil
}

// PreviewFont installs src as a dirty preview; exactly one backup per slot.
func PreviewFont(src, slot string, st *SessionState) (string, error) {
	target, err := checkedTarget(src, slot)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return "", err
	}
	if _, err := EnsureBackupOnce(target); err != nil {
		return "", err
	}
	if st != nil {
		if st.BackedUp == nil {
			st.BackedUp = map[string]bool{}
		}
		st.BackedUp[slot] = true
	}
	if err := copyPreservingMode(src, target); err != nil {
		return "", err
	}
	if st != nil {
		st.Dirty = true
		st.Preview = &PreviewRef{Src: src, Slot: slot}
	}
	ReloadSettings()
	return target, nil
}

// CommitPreview clears the dirty flag, keeping the previewed file.
// It returns "" when there is no preview to commit.
func CommitPreview(st *SessionState) string {
	if st == nil || !st.Dirty || st.Preview == nil {
		return ""
	}
	slot := st.Preview.Slot
	target, err := paths.FontSlotPath(slot)
	if err != nil {
		return ""
	}
	st.Dirty = false
	st.Preview = nil
	return target
}

// RestoreOriginal restores original bytes for the previewed slot (or all
// slots when the preview reference is unknown). It returns false when
// there is nothing to restore.
func RestoreOriginal(st *SessionState) (bool, error) {
	if st == nil || (st.Preview == nil && !st.Dirty) {
		return false, nil
	}
	var slots []string
	if st.Preview != nil {
		slots = []string{st.Preview.Slot}
	} else {
		for slot := range st.Originals {
			slots = append(slots, slot)
		}
		sort.Strings(slots)
	}
	for _, slot := range slots {
		if _, ok := paths.SlotFiles[slot]; !ok {
			continue
		}
		target, err := paths.FontSlotPath(slot)
		if err != nil {
			continue
		}
		original := st.Originals[slot]
		if original == nil {
			if err := os.Remove(target); err != nil && !os.IsNotExist(err) {
				return false, err
			}
		} else {
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return false, err
			}
			if err := os.WriteFile(target, original, 0o644); err != nil {
				return false, err
			}
		}
		delete(st.BackedUp, slot)
	}
	st.Dirty = false
	st.Preview = nil
	ReloadSettings()
	return true, nil
}

// ReloadSettings asks Termux to reload settings; it returns false when
// the termux-reload-settings binary is missing or fails.
func ReloadSettings() bool {
	binary, err := exec.LookPath("termux-reload-settings")
	if err != nil {
		setLastReload(false)
		return false
	}
	if err := exec.Command(binary).Run(); err != nil {
		setLastReload(false)
		return false
	}
	setLastReload(true)
	return true
}

func setLastReload(ok bool) {
	lastReloadOK = &ok
}

// LastReloadOK returns the last ReloadSettings result (nil if never run).
func LastReloadOK() *bool {
	return lastReloadOK
}

// IsPreviewDirty returns true when a preview is awaiting commit/restore.
func IsPreviewDirty(st *SessionState) bool {
	return st != nil && st.Dirty
}
