# Go Bubble Tea Port Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Port `termux-fonts` to Go with a Bubble Tea TUI on branch `go-bubbletea`, behavior-compatible with the Python TUI.

**Architecture:** Clean rewrite on branch `go-bubbletea` forked from `main` (085b5da). UI-free `internal/` packages (paths/scan/validate/apply/importer/downloader) plus one Bubble Tea `tui` package and a `cmd/termux-fonts` CLI entry. Python spec stays the behavioral authority.

**Tech Stack:** Go >= 1.23 (device has 1.27.1 android/arm64), `charmbracelet/bubbletea` + `bubbles` + `lipgloss`, `golang.org/x/image` (sfnt only), stdlib for the rest.

**Spec:** `docs/superpowers/specs/2026-10-06-go-bubbletea-port-design.md` (behavior authority: `docs/superpowers/specs/2026-10-04-font-changer-design.md`)

## Global Constraints

- Go `>= 1.23`.
- External deps: `charmbracelet/bubbletea`, `charmbracelet/bubbles`, `charmbracelet/lipgloss`, `golang.org/x/image` — stdlib for everything else.
- All paths via `TERMUX_HOME` override; tests never touch real `~/.termux` (use `t.Setenv("TERMUX_HOME", t.TempDir())`).
- `internal/` packages MUST NOT import bubbletea/bubbles/lipgloss.
- Backup naming: `<slotbase>-YYYY-MM-DD-HHMMSS.ttf` under `~/.termux/backups/`; one backup per slot per session; reuse match anchored to same slot.
- Reload via `exec.LookPath("termux-reload-settings")` + run, no shell; missing binary = warning, file kept.
- Slots: `regular→font.ttf`, `bold→font-bold.ttf`, `italic→font-italic.ttf`, `bold-italic→font-bold-italic.ttf`.
- Valid magic: `00 01 00 00`, `OTTO`, `true`, `typ1` + sfnt parse check.
- Manual preview only: highlight never applies; Space/p previews, Enter commits, Esc restores, list focused on launch.
- All implementation work happens on branch `go-bubbletea` (Task 1 creates it).

## Review Focus

- Corrupt file with valid magic but broken sfnt tables → rejected before any copy, current font untouched. Pinned by `TestIsValidFont_RejectsCorruptTables` in Task 2.
- Rapid re-preview across two slots → exactly one backup per slot, last preview wins per slot. Pinned by `TestPreview_MultiSlotIndependent` in Task 3.
- Missing `termux-reload-settings` → file installed, manual-restart hint surfaced, no error. Pinned by `TestReload_MissingBinaryWarns` in Task 3.
- Empty library with existing `font.ttf` → seeded as `fonts/Current.ttf`, list non-empty. Pinned by `TestSeed_EmptyLibrary` in Task 6.
- Download server without Content-Length → full download fallback, no false skip. Pinned by `TestFetch_NoContentLengthDownloads` in Task 4.

---

### Task 1: Branch + module + paths

**Files:**
- Create (on branch `go-bubbletea`): `go.mod`, `internal/paths/paths.go`, `internal/paths/paths_test.go`

**Interfaces:**
- Consumes: none.
- Produces:
  - `paths.TermuxDir() string`
  - `paths.FontsDir() string`
  - `paths.FontSlotPath(slot string) (string, error)` (unknown slot → error)
  - `paths.BackupsDir() string`
  - `paths.SlotFiles map[string]string`

- [ ] **Step 1: Create branch and failing test**

```bash
git checkout -b go-bubbletea
```

```go
func TestTermuxHomeOverride(t *testing.T) {
    dir := t.TempDir()
    t.Setenv("TERMUX_HOME", dir)
    if got := paths.FontsDir(); got != filepath.Join(dir, "fonts") { t.Fail() }
    if got, _ := paths.FontSlotPath("regular"); got != filepath.Join(dir, "font.ttf") { t.Fail() }
    if got, _ := paths.FontSlotPath("bold"); got != filepath.Join(dir, "font-bold.ttf") { t.Fail() }
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/paths/ -run TestTermuxHomeOverride -v`
Expected: FAIL (no Go files).

- [ ] **Step 3: Implement `go.mod` (`module termux-fonts-go`, `go >= 1.23`) and `paths.go` with `SLOT_FILES`, `TermuxDir/FontsDir/FontSlotPath/BackupsDir`**

`TERMUX_HOME` wins, else `$HOME/.termux`.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./... -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add go.mod internal/paths/
git commit -m "feat(go): add TERMUX_HOME-aware paths and module"
```

### Task 2: Validate + scan

**Files:**
- Create: `internal/validate/validate.go`, `internal/validate/validate_test.go`
- Create: `internal/scan/scan.go`, `internal/scan/scan_test.go`

**Interfaces:**
- Consumes: `paths.FontsDir()`, `paths.FontSlotPath()`, `paths.SlotFiles`.
- Produces:
  - `validate.ValidMagic [][]byte`
  - `validate.IsValidFont(path string) (bool, string)`
  - `scan.FontEntry struct{Name, Path string; Size int64; Mtime time.Time; Family, Style string}`
  - `scan.ListLibrary() ([]FontEntry, error)` (sorted case-insensitive)
  - `scan.ReadActive() (map[string]string, error)` (missing slot → absent key)

- [ ] **Step 1: Write failing tests**

```go
func TestListLibrary_Sorted(t *testing.T) // names sorted case-insensitively
func TestReadActive_MissingSlot(t *testing.T) // bold absent when file missing
func TestIsValidFont_RejectsGarbage(t *testing.T) // "not a font" → false
func TestIsValidFont_RejectsCorruptTables(t *testing.T) // magic + zeros → false
func TestListLibrary_Empty(t *testing.T) // missing dir → empty, nil error
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/validate/ ./internal/scan/ -v`
Expected: FAIL.

- [ ] **Step 3: Implement `validate.go` (magic check + `sfnt.Parse` forcing table read) and `scan.go` (glob `*.ttf|*.otf` case variants, `sfnt` name IDs 16→1 / 17→2 with stem/`Regular` fallback)**

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./... -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/validate/ internal/scan/
git commit -m "feat(go): add font validation and library scan"
```

### Task 3: Apply core

**Files:**
- Create: `internal/apply/apply.go`, `internal/apply/apply_test.go`

**Interfaces:**
- Consumes: `paths.FontSlotPath/BackupsDir`, `validate.IsValidFont`.
- Produces:
  - `apply.NewSessionState() *SessionState` (`Originals map[string][]byte`, `BackedUp map[string]bool`, `Dirty bool`, `Preview *PreviewRef`)
  - `apply.EnsureBackupOnce(target string) (string, error)` ("" when target missing)
  - `apply.InstallFont(src, slot string) (string, error)`
  - `apply.PreviewFont(src, slot string, st *SessionState) (string, error)`
  - `apply.CommitPreview(st *SessionState) string`
  - `apply.RestoreOriginal(st *SessionState) (bool, error)`
  - `apply.ReloadSettings() bool`
  - `apply.LastReloadOK() *bool`, `apply.ManualRestartHint string`
  - `apply.IsPreviewDirty(st *SessionState) bool`

Reuse glob `filepath.Glob(dir/<stem>-[0-9][0-9][0-9][0-9]-*<suffix>)` plus full-match check `^{stem}-\d{4}-\d{2}-\d{2}-\d{6}{suffix}$` (regexp-quoted) — the anchored per-slot algorithm from the Python fix.

- [ ] **Step 1: Write failing tests**

```go
func TestBackupOnce_Reused(t *testing.T) // two previews same slot → 1 backup
func TestPreview_MultiSlotIndependent(t *testing.T) // bold then regular → 2 backups
func TestCommit_ClearsDirty(t *testing.T)
func TestRestore_AfterPreview(t *testing.T) // original bytes back, dirty cleared
func TestPreview_InvalidRejected(t *testing.T)
func TestReload_MissingBinaryWarns(t *testing.T) // PATH without binary → false, file kept
```

Build real minimal TTF fixtures in-test (or commit 2 tiny fixtures under `testdata/`).

- [ ] **Step 2: Run to verify they fail**

Run: `go test ./internal/apply/ -v`
Expected: FAIL.

- [ ] **Step 3: Implement `apply.go`**

Timestamp `time.Now().Format("2006-01-02-150405")`; copy with mode preservation; reload via `exec.LookPath` + `exec.Command(bin).Run()`, all errors → false.

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./... -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/apply/
git commit -m "feat(go): add backup-once preview/commit/restore and reload"
```

### Task 4: Importer + downloader

**Files:**
- Create: `internal/importer/importer.go`, `internal/importer/importer_test.go`
- Create: `internal/downloader/downloader.go`, `internal/downloader/downloader_test.go`

**Interfaces:**
- Consumes: `paths.FontsDir()`, `validate.IsValidFont`.
- Produces:
  - `importer.ImportFile(src, clash string) (string, error)` (`clash` ∈ `error|keep-both|replace`)
  - `importer.ResolveClash(dest string) string` (`name-N.ttf`)
  - `downloader.NerdFonts map[string]string` (same 11 URLs as Python)
  - `downloader.Fetch(name string, force bool) (string, error)`

- [ ] **Step 1: Write failing tests**

```go
func TestImport_CopiesValid(t *testing.T)
func TestImport_ClashKeepBoth(t *testing.T) // → Hack-1.ttf
func TestImport_InvalidRejected(t *testing.T)
func TestFetch_SkipsIfSameSize(t *testing.T) // httptest server, handler counts hits → 0
func TestFetch_NoContentLengthDownloads(t *testing.T) // no Content-Length → downloads
```

- [ ] **Step 2: Run to verify they fail**

Run: `go test ./internal/importer/ ./internal/downloader/ -v`
Expected: FAIL.

- [ ] **Step 3: Implement `importer.go` (validate → clash → copy) and `downloader.go` (HEAD size check, GET to temp + `os.Rename`, cleanup on error)**

EACCES (or permission-denied reason) → error mentioning `termux-setup-storage`.

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./... -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/importer/ internal/downloader/
git commit -m "feat(go): add font import and Nerd Font download"
```

### Task 5: Bubble Tea TUI

**Files:**
- Create: `internal/tui/model.go`, `internal/tui/model_test.go` (keep one file unless >400 lines)

**Interfaces:**
- Consumes: all `internal/` packages.
- Produces:
  - `tui.NewModel() Model`
  - `tui.InitialModel() Model` (for `tea.NewProgram`)
  - Keymap: space/p preview, enter commit, s cycle slot, i import overlay, d download overlay, esc restore-then-back, q quit (restore if dirty)
  - Focus: list focused on Init (arrows move immediately; Tab reaches filter)

Layout: Bubbles `list` (42%) + viewport/static preview (58%), `textinput` filter, Lip Gloss borders. Highlight updates info only. Downloads run as `tea.Cmd` returning messages; all I/O errors become status-line text.
Visual style (spec §8): gradient title bar, rounded borders + adaptive accent, rich preview pane (large sample + coverage row + slot/backup status), filter match highlight, spinner + progress on downloads, styled help bar, accent-background selection. Palette restrained to accent + muted + foreground.

- [ ] **Step 1: Write failing test (model-level, no TTY)**

```go
func TestModel_FilterNarrowsList(t *testing.T) // Update(filterMsg) shrinks visible items
func TestModel_HighlightIsSideEffectFree(t *testing.T) // cursor move → slot bytes unchanged
func TestPreviewView_ShowsSampleAndCoverage(t *testing.T) // preview pane contains Aa sample + coverage row + slot/backup status
func TestDownload_ShowsProgress(t *testing.T) // progress messages update model 0→1, done dismisses
```

- [ ] **Step 2: Run to verify they fail**

Run: `go test ./internal/tui/ -v`
Expected: FAIL.

- [ ] **Step 3: Implement `model.go` (Model/Update/View, Bubbles list+textinput, Lip Gloss 42/58)**

Pin `charmbracelet/bubbletea`, `bubbles`, `lipgloss` versions in `go.mod` at first working build (`go mod tidy`).

- [ ] **Step 4: Run full suite + `go vet`**

Run: `go test ./... -v && go vet ./...`
Expected: PASS, clean vet.

- [ ] **Step 5: Commit**

```bash
git add internal/tui/ go.mod go.sum
git commit -m "feat(go): add Bubble Tea font picker with manual live preview"
```

### Task 6: CLI, seed, docs

**Files:**
- Create: `cmd/termux-fonts/main.go`
- Create: `README-GO.md` (or `README.md` on branch)
- Test: extend `internal/tui` or new `cmd/termux-fonts/main_test.go` for flag parsing + seed

**Interfaces:**
- Consumes: all packages.
- Produces: `termux-fonts [--list] [--apply NAME --slot SLOT]` (slot default `regular`), builtin seed (empty library + `font.ttf` exists → copy to `fonts/Current.ttf`).

- [ ] **Step 1: Write failing tests**

```go
func TestSeed_EmptyLibrary(t *testing.T) // font.ttf present, fonts/ empty → Current.ttf created
```

(Flag parsing tested via extracted `resolveMatch(entries, name)` helper returning (entry, ambiguous bool, found bool).)

- [ ] **Step 2: Run to verify they fail**

Run: `go test ./cmd/... ./internal/tui/ -v`
Expected: FAIL.

- [ ] **Step 3: Implement `main.go` (flag pkg, `--list` prints names, `--apply` exact-then-casefold match, seed before both TUI and CLI paths) + README (build `go build ./...`, usage keys, backup/seed notes)**

- [ ] **Step 4: Run full suite + on-device build smoke**

Run: `go test ./... -v && go build -o /tmp/termux-fonts-go ./cmd/termux-fonts && TERMUX_HOME=/tmp/fc-go-demo /tmp/termux-fonts-go --list`
Expected: PASS + exit 0.

- [ ] **Step 5: Commit**

```bash
git add cmd/ README-GO.md
git commit -m "feat(go): add CLI flags, builtin seed, and docs"
```
