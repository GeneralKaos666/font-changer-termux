# Review-Findings Fixes Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Fix every finding of the 2026-10-09 whole-repo code review: two TUI bugs (q-trap on failed restore, commit toast naming the wrong slot), the missing import clash prompt / `~` expansion / tab-complete, the dead `downloadProgressMsg`, non-atomic slot writes, slot-list sprawl across three packages, duplicated helpers, the 651-line `model.go`, and align the Go spec with the shipped (user-approved) 46-family catalog, shell-prompt row, and glyph metadata.

**Architecture:** Nine ordered tasks, one package-focused each: docs first (bless approved divergences), then the two small TUI bug fixes, then path/slot consolidation, micro-refactors, atomic apply copy, the import UX feature, measured download progress, and finally the mechanical `model.go` file split. Every task ends with `go build ./... && go test ./... && go vet ./... && gofmt -l .` green and its own commit.

**Tech Stack:** Go >= 1.26 (go.mod), Bubble Tea (`bubbletea`/`bubbles`/`lipgloss` confined to `internal/tui` + `cmd` entrypoint), `golang.org/x/image/font/sfnt`, stdlib only elsewhere (`net/http`, `os/exec`, `io`).

**Spec:** `docs/superpowers/specs/2026-10-06-go-bubbletea-port-design.md` (primary; conflicts resolve against `docs/superpowers/specs/2026-10-04-font-changer-design.md`). Task 1 amends both per the user's product decisions of 2026-10-09.

## Global Constraints

- `TERMUX_HOME` wins over `$HOME/.termux` for every Termux config path; tests must `t.Setenv("TERMUX_HOME", t.TempDir())` and never touch the real `~/.termux`.
- Only `internal/tui` may import bubbletea/bubbles/lipgloss; `cmd/termux-fonts/main.go` may import bubbletea to start the program (Task 1 clarifies this in AGENTS.md).
- Every filesystem location goes through `internal/paths`; never hardcode `~/.termux` (or `TermuxDir() + "/colors.properties"` — use `paths.ColorsPath()`).
- Slots `regular|bold|italic|bold-italic` → `font.ttf | font-bold.ttf | font-italic.ttf | font-bold-italic.ttf`. Unknown slot is an error, never a fallback.
- Font validity = magic bytes + sfnt parse, enforced before any copy.
- Backups: `{stem}-YYYY-MM-DD-HHMMSS{suffix}` under `~/.termux/backups/`, anchored reuse (Python parity — reuse is filename-scoped, not strictly per-session).
- Preview lifecycle: `PreviewFont` → `CommitPreview` or `RestoreOriginal`; highlighting alone must never install a font.
- Tests are hermetic by construction: no real network, no real shell, no real `$HOME`; downloader tests use `httptest` and mutate the exported `NerdFonts` map; TUI tests stub `capturePromptLines` in `TestMain` and use the `noReload` PATH stub.
- Nerd Fonts catalog pinned to `nfBase` v3.2.1; SymbolsOnly/Propo excluded; `JetBrainsMono-Light` retained deliberately.
- Checks: `go build ./...`, `go test ./...`, `go vet ./...`, `gofmt -l .` (must print nothing).

## Review Focus

Inputs and conditions the specs imply but no current test pins — each gets its test in the owning task:

1. Import of a missing/nonexistent path → status error, overlay closes, library untouched. (Task 7)
2. Import when the destination already exists → three-choice prompt (keep-both / replace / cancel); cancel leaves the library untouched; never a silent rename. (Task 7)
3. `q` with a failed restore → the app still quits and surfaces the failure; `esc` with a failed restore stays open and surfaces it. (Task 2)
4. Commit after cycling the slot off the previewed one → the status names the committed (preview) slot, never `m.slot`. (Task 3)
5. A slot file that is a symlink → preview/install replace the symlink's target and keep the symlink (write-through preserved). (Task 6)
6. Download with unknown Content-Length (chunked/streamed) → the bar stays an activity indicator with the "no progress info" wording; measured fraction only when the total is known. (Task 8)
7. `~` in an import path expands to the real `$HOME` even when `TERMUX_HOME` points elsewhere. (Task 7)

---

### Task 1: Bless shipped features in the specs and AGENTS.md

**Files:**
- Modify: `docs/superpowers/specs/2026-10-06-go-bubbletea-port-design.md`
- Modify: `docs/superpowers/specs/2026-10-04-font-changer-design.md` (only the "out of scope" line, to name the Go-only exceptions — see step 3)
- Modify: `AGENTS.md` (gitignored, at repo root)
- None (docs only; no tests)

**Interfaces:**
- Produces: the documented authority Task 2–9 argue from; no code.

- [ ] **Step 1: Amend the Go port spec's non-goal line**

In `2026-10-06-go-bubbletea-port-design.md` §1, replace the YAGNI non-goal ("no new manager features") with an exceptions clause approving, as deliberate Go-only additions: the full 46-family Nerd Font catalog with the two-column filtered download picker; the live shell-prompt row in the preview pane (captures `$SHELL -ic` once per session, ANSI-safe, mock fallback); and the glyph/UPM/version metadata line.

- [ ] **Step 2: Amend the Go port spec's component descriptions**

- §3.5: note the import flow gains a three-choice clash prompt (keep-both / replace / cancel), `~` expansion, and tab-complete of `$HOME` paths (deferred in the Python TUI, approved here).
- §3.6: change "`NERD_FONTS` (same 11 URLs)" to "full 46-family `NERD_FONTS` (one Regular per family plus `JetBrainsMono-Light`), pinned to the v3.2.1 `nfBase`".
- §3.2: add `internal/theme` (`colors.properties` palette → adaptive accent/muted) and note `scan.Describe` drives the glyph/UPM/version line.
- §8: add the shell-prompt line and the glyph/UPM/version info line to the preview-pane list.

- [ ] **Step 3: Cross-reference the Python spec's out-of-scope line**

In `2026-10-04-font-changer-design.md` §6 "Out of scope", append one line: "Go port adds: full Nerd catalog + download picker, live shell-prompt row, glyph/UPM metadata, palette theming (see Go port spec §1)."

- [ ] **Step 4: Fix the stale AGENTS.md wording**

- "one Regular per family" catalog note → "one Regular per family plus the deliberate `JetBrainsMono-Light` second weight (mirrors fonts.sh)".
- "one per slot per session" backup line → "one timestamped backup per slot, anchored reuse (filename-scoped, Python parity)".
- The tui-import hard rule → "`internal/tui` is the only package allowed to import bubbletea/bubbles/lipgloss; `cmd/termux-fonts/main.go` may import bubbletea to start the program."

- [ ] **Step 5: Verify and commit**

Run: `git diff --stat` and eyeball the two spec diffs + AGENTS.md.
Commit: `docs: bless full Nerd catalog, shell prompt, glyph metadata; clarify import rule and backup wording`

---

### Task 2: `q` always quits, even when the restore fails

**Files:**
- Modify: `internal/tui/model.go:555-563`
- Test: `internal/tui/model_test.go` (rewrite `TestQuit_RestoreFailureStaysOpen`)

**Interfaces:**
- Consumes: `apply.IsPreviewDirty(st *apply.SessionState) bool`, `restoreOriginal` (package var in tui).
- Produces: none (behavior change only).

- [ ] **Step 1: Write the failing test**

Replace `TestQuit_RestoreFailureStaysOpen` (model_test.go:185-222) with `TestQuit_RestoreFailureStillQuits`:

```go
func TestQuit_RestoreFailureStillQuits(t *testing.T) {
	useTermuxHome(t)
	noReload(t)
	seedLibrary(t, "Hack.ttf")

	m := NewModel()
	m = updateModel(t, m, keyRunes(" "))     // preview → dirty
	old := restoreOriginal
	restoreOriginal = func(*apply.SessionState) (bool, error) { return false, errors.New("boom") }
	defer func() { restoreOriginal = old }()

	next, cmd := m.Update(keyRunes("q"))     // q with failed restore
	m = next.(Model)
	if cmd == nil {
		t.Fatal("q with failed restore must still quit")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatalf("q cmd returned %T, want tea.QuitMsg", cmd())
	}
	if !strings.Contains(m.status, "Restore failed") {
		t.Fatalf("status = %q, want restore failure text", m.status)
	}
}
```

- [ ] **Step 2: Run it and confirm it fails**

Run: `go test ./internal/tui -run TestQuit_RestoreFailureStillQuits -v`
Expected: FAIL — the current code returns `m, nil` on restore error, so `cmd == nil`.

- [ ] **Step 3: Make `q` quit unconditionally**

In `handleKey` case `"q"` (model.go:555-563): when dirty, attempt `restoreOriginal`; on error set `m.status = "Restore failed: " + err.Error() + " — quitting anyway"` and do NOT return early; fall through to `return m, tea.Quit` either way. (`esc` keeps its current stay-open-on-error behavior.)

- [ ] **Step 4: Run the full suite**

Run: `go build ./... && go test ./... && go vet ./... && test -z "$(gofmt -l .)"`
Expected: all green (the old `TestQuit_RestoreFailureStaysOpen` no longer exists; no other test asserts the q-trap).

- [ ] **Step 5: Commit**

```bash
git add internal/tui/model.go internal/tui/model_test.go
git commit -m "fix(tui): q quits even when the dirty-restore fails (matches Python quit semantics)"
```

---

### Task 3: Commit toast names the committed (preview) slot

**Files:**
- Modify: `internal/tui/actions.go:59-68` (`doCommit`)
- Test: `internal/tui/model_test.go`

**Interfaces:**
- Consumes: `apply.IsPreviewDirty`, `apply.CommitPreview(st)`, `st.Preview.Slot` (read BEFORE `CommitPreview` clears `Preview`); `slotOrder` (Task 4 renames this to `paths.SlotNames` — this task still compiles against the current name).
- Produces: none.

- [ ] **Step 1: Write the failing test**

```go
func TestCommit_ToastNamesCommittedSlot(t *testing.T) {
	useTermuxHome(t)
	noReload(t)
	seedLibrary(t, "Hack.ttf")

	m := NewModel()
	m = updateModel(t, m, keyRunes(" ")) // preview into slot "regular"
	m = updateModel(t, m, keyRunes("s")) // cycle slot to "bold"
	m = updateModel(t, m, keyRunes("enter"))
	if got := m.status; !strings.Contains(got, "Kept Hack.ttf → regular slot") {
		t.Fatalf("commit status = %q, want it to name the committed regular slot", got)
	}
	if strings.Contains(m.status, "bold slot") {
		t.Fatalf("commit status names the current slot instead of the committed one: %q", m.status)
	}
}
```

- [ ] **Step 2: Run it and confirm it fails**

Run: `go test ./internal/tui -run TestCommit_ToastNamesCommittedSlot -v`
Expected: FAIL — current message reads "→ bold slot".

- [ ] **Step 3: Read the preview slot before committing**

In `doCommit` (actions.go:60-67), when dirty: `slot := m.state.Preview.Slot` first, then `target := apply.CommitPreview(m.state)` (which nils `Preview`), then `m.status = fmt.Sprintf("Kept %s → %s slot", filepath.Base(target), slot) + reloadHint()`. The non-dirty `InstallFont` branch keeps using `m.slot` (correct there).

- [ ] **Step 4: Run the full suite**

Run: `go build ./... && go test ./... && go vet ./... && test -z "$(gofmt -l .)"`
Expected: all green.

- [ ] **Step 5: Commit**

```bash
git add internal/tui/actions.go internal/tui/model_test.go
git commit -m "fix(tui): commit toast names the previewed slot, not the current one"
```

---

### Task 4: One slot list + one colors path (kill the slot sprawl)

**Files:**
- Modify: `internal/paths/paths.go`
- Modify: `internal/tui/model.go:107` (delete `slotOrder`), `internal/tui/actions.go:29-37` (`cycleSlot`), `internal/tui/model.go:114` (`themePalette`)
- Modify: `cmd/termux-fonts/main.go:81-88,139` (delete `slotNames()`, replace slot lookup)
- Modify: `internal/apply/apply.go:132-141` (`checkedTarget`)
- Test: `internal/paths/paths_test.go`, `internal/tui/model_test.go`, `cmd/termux-fonts/main_test.go`

**Interfaces:**
- Consumes: existing `paths.FontSlotPath(slot) (string, error)` (single source of slot validity).
- Produces: `paths.SlotNames` — `[]string{"regular","bold","italic","bold-italic"}` (canonical s-key cycle order; a var, since a map can't be ordered); `paths.ColorsPath() string` → `TermuxDir() + "/colors.properties"`.

- [ ] **Step 1: Add `SlotNames` and `ColorsPath` to paths.go with tests**

```go
// SlotNames is the canonical slot order — the s-key cycle and the CLI
// help text both derive from it.
var SlotNames = []string{"regular", "bold", "italic", "bold-italic"}

// ColorsPath is the Termux colors.properties file.
func ColorsPath() string { return filepath.Join(TermuxDir(), "colors.properties") }
```

In `paths_test.go`:
- `SlotNames` equals `["regular","bold","italic","bold-italic"]` and its elements are exactly the keys of `SlotFiles`.
- `ColorsPath() == filepath.Join(TermuxDir(), "colors.properties")` under a temp `TERMUX_HOME`.

Run: `go test ./internal/paths -v` — green.

- [ ] **Step 2: Point tui at the canonical sources**

- Delete `var slotOrder` (model.go:106-107); `cycleSlot` (actions.go:29-37) iterates `paths.SlotNames` (len-1 wrap unchanged).
- `themePalette` (model.go:113-119): `theme.LoadFile(paths.ColorsPath())` instead of `paths.TermuxDir() + "/colors.properties"`.
- Add tui test `TestModel_CycleSlot`: seed, `NewModel()`, `keyRunes("s")` → `m.slot == "bold"`; four `s` presses return to `"regular"`.

Run: `go test ./internal/tui -run 'TestModel_CycleSlot|TestTheme' -v` — green.

- [ ] **Step 3: Point main.go at the canonical sources**

- Delete `slotNames()` (main.go:81-88). The error at main.go:139-141 becomes:
  `if _, err := paths.FontSlotPath(*slot); err != nil { fmt.Fprintf(os.Stderr, "%v (choose from %s)\n", err, strings.Join(paths.SlotNames, ", ")); return 2 }`
  (message text is unchanged; the listed order changes from alphabetical to cycle order — intended).
- Add main_test `TestRealMain_UnknownSlotIsExit2`: `realMain(["--apply","Hack.ttf","--slot","bogus"]) == 2`.

Run: `go test ./cmd/termux-fonts -v` — green.

- [ ] **Step 4: Drop the duplicate check in apply**

In `checkedTarget` (apply.go:132-141): delete the `if _, ok := paths.SlotFiles[slot]; !ok { ... }` block; `paths.FontSlotPath(slot)` (already called at the end) is the sole validity source and errors identically on unknown slots (error text changes from `unknown slot: %q` to `unknown font slot: %q` — no test asserts the old text; verify with `go test ./internal/apply`).

- [ ] **Step 5: Run the full suite**

Run: `go build ./... && go test ./... && go vet ./... && test -z "$(gofmt -l .)"`
Expected: all green.

- [ ] **Step 6: Commit**

```bash
git add internal/paths internal/tui cmd/termux-fonts internal/apply
git commit -m "refactor: canonical paths.SlotNames and paths.ColorsPath; slots validated in one place"
```

---

### Task 5: Three micro-refactors (match-hits, entry summary, Base)

**Files:**
- Modify: `cmd/termux-fonts/main.go` (`resolveMatch`, `runApply` ambiguous block)
- Modify: `internal/tui/view.go:108,199` (duplicated family·style·size string)
- Modify: `internal/tui/actions.go:16` (delete `baseName`), `internal/tui/model.go:412,423` (callers)
- Test: `cmd/termux-fonts/main_test.go`, `internal/tui/model_test.go`

**Interfaces:**
- Produces: `casefoldHits(entries []scan.FontEntry, name string) []scan.FontEntry` (main.go) and `entrySummary(e scan.FontEntry) string` (tui view.go) — both package-private.
- Consumes: `paths.SlotNames` (already, from Task 4 — no new coupling).

- [ ] **Step 1: Extract `casefoldHits` and reuse it**

- Add `func casefoldHits(entries []scan.FontEntry, name string) []scan.FontEntry` returning every entry whose `Name` equals `name` case-insensitively (the loop currently inline in `resolveMatch`:65-70).
- `resolveMatch` calls it; the ambiguous branch of `runApply` (main.go:102-111) replaces its re-loop with `names` from `casefoldHits(entries, name)`.
- Add main_test: `casefoldHits(entries("B.ttf","b.TTF","A.ttf"), "b.ttf")` returns exactly `["B.ttf","b.TTF"]`.

Run: `go test ./cmd/termux-fonts -v` — green (existing `resolveMatch` tests keep passing).

- [ ] **Step 2: Extract `entrySummary` in view.go**

- Add `func entrySummary(e scan.FontEntry) string { return fmt.Sprintf("%s · %s · %s", e.Family, e.Style, humanSize(e.Size)) }`.
- Use it at view.go:108 (`descPlain := truncateCells(entrySummary(fi.entry), d.width)` path) and view.go:199.

Run: `go test ./internal/tui -run 'TestPreviewView|TestPreviewPane' -v` — green (output unchanged).

- [ ] **Step 3: Delete `baseName`**

- Remove `baseName` (actions.go:16); call `filepath.Base` at actions.go:66, actions.go:79, model.go:412, model.go:423. Drop the now-unused `path/filepath` import from actions.go and add it to model.go.

Run: `go build ./... && go test ./... && go vet ./... && test -z "$(gofmt -l .)"` — green.

- [ ] **Step 4: Commit**

```bash
git add cmd/termux-fonts internal/tui
git commit -m "refactor: reuse casefoldHits and entrySummary; inline filepath.Base"
```

---

### Task 6: Atomic slot writes that survive symlinks

**Files:**
- Modify: `internal/apply/apply.go:116-130` (`copyPreservingMode` only)
- Test: `internal/apply/apply_test.go`

**Interfaces:**
- Consumes: none new.
- Produces: unchanged signature `copyPreservingMode(src, dst string) error` — behavior: `dst` may be a symlink whose target is what actually gets replaced (write-through preserved); the copy lands via a temp sibling + `os.Rename`, so an interrupted copy never leaves a truncated live slot.

- [ ] **Step 1: Write the symlink guard test (passes today, pins the semantics)**

In `apply_test.go`:

```go
func TestInstallFont_KeepsSymlinkSlot(t *testing.T) {
	root := useTermuxHomeShape(t) // TERMUX_HOME temp + noReload-style PATH stub, as in existing tests
	target := filepath.Join(root, "real-font.ttf")
	if err := os.WriteFile(target, fixtureBytes(t, "a.ttf"), 0o644); err != nil {
		t.Fatal(err)
	}
	slot, err := paths.FontSlotPath("regular")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, slot); err != nil {
		t.Fatal(err) // skip if symlinks unsupported (e.g. FAT)
	}
	fixture := filepath.Join("testdata", "b.ttf")
	if _, err := apply.InstallFont(fixture, "regular"); err != nil {
		t.Fatal(err)
	}
	if fi, err := os.Lstat(slot); err != nil || fi.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("slot is no longer a symlink after install: %v %v", fi, err)
	}
	got, _ := os.ReadFile(target) // writes must land in the real file, through the symlink
	want, _ := os.ReadFile(fixture)
	if string(got) != string(want) {
		t.Fatal("slot target was not replaced by the installed font")
	}
}
```

(Match the existing test helpers' shape — `useTermuxHome`/`noReload` live in `internal/tui/model_test.go`, so replicate their two `t.Setenv` lines inside apply_test as needed.)

Run: `go test ./internal/apply -run TestInstallFont_KeepsSymlinkSlot -v` — PASS today (os.WriteFile follows the symlink).

- [ ] **Step 2: Rework `copyPreservingMode` to temp + rename with symlink resolution**

Approach: `resolved := dst; if r, err := filepath.EvalSymlinks(dst); err == nil { resolved = r }`; `os.CreateTemp(filepath.Dir(resolved), ".apply-*.part")`; `io.Copy` from a fresh `os.Open(src)`; close; `os.Chmod(tmp, fi.Mode().Perm())`; `os.Rename(tmp, resolved)`; `defer os.Remove(tmp)` on any error. `EnsureBackupOnce` (fresh unique dest → EvalSymlinks fails → raw dest) is unaffected.

- [ ] **Step 3: Run the guard test plus the whole package**

Run: `go test ./internal/apply -v`
Expected: `TestInstallFont_KeepsSymlinkSlot` and all existing copy/backup/preview/restore tests green.

- [ ] **Step 4: Run the full suite**

Run: `go build ./... && go test ./... && go vet ./... && test -z "$(gofmt -l .)"`
Expected: all green.

- [ ] **Step 5: Commit**

```bash
git add internal/apply/apply.go internal/apply/apply_test.go
git commit -m "fix(apply): atomic temp+rename slot writes, preserving symlink write-through"
```

---

### Task 7: Import UX — `~` expansion, clash prompt, tab-complete

**Files:**
- Modify: `internal/importer/importer.go` (new `ClashError`, `ImportFile` mode `"ask"`)
- Modify: `internal/tui/actions.go` (`importCmd(path, clash)`, `expandHome`, `completePath`)
- Modify: `internal/tui/model.go` (overlay enum + `pendingImportPath` field; `overlayImport` key handling; `importDoneMsg` handler routes `ClashError` to the prompt; tab key)
- Modify: `internal/tui/view.go` (render the clash-prompt box)
- Test: `internal/importer/importer_test.go`, `internal/tui/model_test.go`

**Interfaces:**
- Produces:
  - `importer.ClashError struct { Dest string }` with `func (e *ClashError) Error() string`; `errors.As`-detectable.
  - `importer.ImportFile(src, clash string)` accepts mode `"ask"`: on existing destination (not the same file) returns `*ClashError` instead of importing; other modes unchanged.
  - `importCmd(path, clash string) tea.Cmd` (tui) — sends `importDoneMsg`; when the error is a `*ClashError`, the `importDoneMsg` handler switches to the clash prompt instead of reporting failure.
  - `expandHome(p string) string` and `completePath(s string) string` (tui, pure).
  - `overlayImportClash` overlay value + `pendingImportPath string` field.
- Consumes: `importCmd` call site moves to `handleKey` (model.go:464).

- [ ] **Step 1: Add `ClashError` and `"ask"` mode to importer with tests**

```go
type ClashError struct{ Dest string }

func (e *ClashError) Error() string { return "font already in library: " + filepath.Base(e.Dest) }
```

`ImportFile(src, "ask")`: when `os.Stat(dest)` succeeds and `!sameFile(src, dest)`, return `&ClashError{Dest: dest}` (before writing); `sameFile` still short-circuits to `dest, nil`. Tests in `importer_test.go`:
- `ImportFile(valid, "ask")` with a pre-existing same-named library file → `errors.As` yields `*ClashError` with `Dest == <fonts>/Name.ttf`, and the library file is untouched.
- `ImportFile(valid, "ask")` with no clash → imports (dest returned), library gains the file.
- `ImportFile(valid, "ask")` where `src` IS the library file → returns `dest`, no clash (sameFile path; pin it).

Run: `go test ./internal/importer -v` — new tests fail before the implementation, pass after.

- [ ] **Step 2: Add `expandHome` and `completePath` (pure) with unit tests**

```go
func expandHome(p string) string {
	if p == "~" || strings.HasPrefix(p, "~/") {
		if home, err := os.UserHomeDir(); err == nil && home != "" {
			if p == "~" {
				return home
			}
			return filepath.Join(home, p[2:])
		}
	}
	return p
}

func completePath(s string) string { /* see step 3 for contract */ }
```

Tests (in `model_test.go` or a new `actions_test.go`): `expandHome("~/x.ttf")` with `t.Setenv("HOME", tmp)` → `tmp/x.ttf`; `expandHome("/abs/x.ttf")` unchanged.

- [ ] **Step 3: Implement `completePath`**

Contract: on a tab press in the import input, return the completed path or the input unchanged:
1. `expandHome` the input first.
2. If it names a directory → append `/`.
3. Else let `dir, base := filepath.Split(s)`; read `os.ReadDir(dir)`; collect names with a case-insensitive `base` prefix; if exactly one → `dir + name`; if more than one → `dir + longestCommonPrefix(names, base)` (a small local helper; empty when none match — return input unchanged).
4. Empty dir (relative path) → no-op.
Unit tests with `t.TempDir()`: one matching entry completes; two entries with a shared prefix complete to the common prefix; no match leaves input untouched.

Run: `go test ./internal/tui -run 'TestExpandHome|TestCompletePath' -v` — green.

- [ ] **Step 4: Route the import flow through the clash prompt**

- Add `overlayImportClash` to the `overlay` enum and `pendingImportPath string` to `Model`.
- `handleKey` `overlayImport` branch: on `"enter"`, call `importCmd(strings.TrimSpace(m.importInput.Value()), "ask")` (and `expandHome` the value first). Add case `"tab"`: `m.importInput.SetValue(completePath(m.importInput.Value()))`.
- `importDoneMsg` handler (model.go:417-427): before the `msg.err != nil` branch, `var ce *importer.ClashError; if errors.As(msg.err, &ce) { m.overlay = overlayImportClash; m.pendingImportPath = msg.path; m.importInput.Blur(); m.status = "File exists — 1 keep both, 2 replace, esc cancel"; return m, nil }`. (Import the `errors` stdlib package.)
- New `handleKey` branch `if m.overlay == overlayImportClash`: `"1"` → `importCmd(m.pendingImportPath, "keep-both")`; `"2"` → `importCmd(m.pendingImportPath, "replace")`; `"esc"` → `m.overlay = overlayNone`, `m.status = "Import cancelled"`, return. Any other key → ignored.

- [ ] **Step 5: Render the clash prompt**

In `View()` (view.go:291-293): when `m.overlay == overlayImportClash`, render a box row: `boxStyle.Width(max(w-4, 30)).Render("File already exists:\n[i] keep both  [r] replace  [esc] cancel")` — then map keys `"i"` → 1 and `"r"` → 2 in the new branch for discoverability (keep `"1"`/`"2"` working too).

- [ ] **Step 6: Add the model-level flow tests**

In `model_test.go` (hermetic: fixtures via `fixtureBytes` written to a temp path OUTSIDE the library):
1. `TestImport_Flow`: `i`, type `tmp/newfont.ttf` (a valid fixture), `enter` → status starts with `"Imported "`, library gains the file.
2. `TestImport_ClashPromptKeepBoth`: pre-write the same basename into the library; import flow → status contains `"File exists"`; press `"1"` → status starts with `"Imported "` and the kept file is `name-1.ttf`.
3. `TestImport_ClashPromptReplace`: same setup, press `"2"` → library file replaced (bytes equal the fixture).
4. `TestImport_ClashPromptCancel`: press `"esc"` → status `"Import cancelled"`, library untouched (only the pre-existing file).
5. `TestImport_MissingPath`: `i`, type nonexistent path, `enter` → status starts with `"Import failed"` and the overlay is gone.
6. `TestImport_HomeExpansion`: `t.Setenv("HOME", tmp)` where `tmp/import.ttf` is a valid fixture, `TERMUX_HOME` pointing elsewhere → `~/import.ttf` imports from the real HOME and lands in TERMUX_HOME's library.

Run: `go test ./internal/tui -run TestImport -v` — green; then the full suite.

- [ ] **Step 7: Run the full suite**

Run: `go build ./... && go test ./... && go vet ./... && test -z "$(gofmt -l .)"`
Expected: all green (existing `TestImport`-adjacent tests untouched; `TestRealMain_*` unaffected).

- [ ] **Step 8: Commit**

```bash
git add internal/importer internal/tui
git commit -m "feat(tui): import clash prompt (keep-both/replace/cancel), ~ expansion, tab-complete"
```

---

### Task 8: Measured download progress (wire up `downloadProgressMsg`)

**Files:**
- Modify: `internal/downloader/downloader.go` (`Fetch` gains a progress hook)
- Modify: `internal/tui/actions.go` (`fetchCmd` becomes `startFetch`, which Task 9 moves to `download.go`), `internal/tui/model.go` (msg shape, `dlCh` field, progress routing, status rewrite)
- Test: `internal/downloader/downloader_test.go`, `internal/tui/model_test.go`

**Interfaces:**
- Produces:
  - `downloader.ProgressFunc func(downloaded, total int64)`; `Fetch(name string, force bool, onProgress ProgressFunc) (string, error)`. Callbacks: each body read with the running totals; called once with `(0, -1)` when the total is unknown (no/negative Content-Length); the final call reaches `(total, total)`.
  - `downloadProgressMsg struct { frac float64; gen int; known bool }` (replaces the bare `float64` type).
  - `dlCh chan tea.Msg` field on `Model`.
- Consumes: existing `skip-if-size` (HEAD) logic and atomic `.part` rename in `Fetch` — unchanged; the hook is additive. All four existing `Fetch(name, false)` test call sites (downloader_test.go:72,78,113,152) pass `nil` where the progress argument was absent.

- [ ] **Step 1: Add the progress hook to `Fetch` and pin it with tests**

Wrap the copy: `io.Copy(tmp, &progressReader{r: resp.Body, total: resp.ContentLength, on: onProgress})` where `progressReader.Read` calls `on(done, total)` after each read; when `total < 0`, call `on(0, -1)` once before copying. `nil onProgress` short-circuits (keep the callback branch cheap). Update the four test call sites to `Fetch("TestSkip", false, nil)` etc.

Tests in `downloader_test.go`:
- `TestFetch_ReportsProgress`: known-length server body (like `TestFetch_SkipsIfSameSize`'s GET handler); collect fractions; assert the callback fired ≥ 2 times, is non-decreasing, and the last call is `(len(data), len(data))` (fraction 1.0).
- `TestFetch_UnknownLengthReportsUnknown`: chunked handler (write in two chunks with `w.(http.Flusher).Flush()` between → `resp.ContentLength == -1`); assert the only callback is `(0, -1)`.

Run: `go test ./internal/downloader -v` — new tests fail before, pass after.

- [ ] **Step 2: Change the message shape and the fetch cmd**

```go
type downloadProgressMsg struct {
	frac  float64
	gen   int
	known bool
}
```

`downloadStartMsg` handler creates the whole pipeline (delete `fetchCmd`; download.go later hosts this as `startFetch(name string, gen int) (chan tea.Msg, tea.Cmd)`):

```go
func startFetch(name string, gen int) (chan tea.Msg, tea.Cmd) {
	ch := make(chan tea.Msg, 8)
	go func() {
		defer close(ch)
		progress := func(done, total int64) {
			if total <= 0 {
				ch <- downloadProgressMsg{known: false, gen: gen}
				return
			}
			ch <- downloadProgressMsg{frac: float64(done) / float64(total), gen: gen, known: true}
		}
		dest, err := downloader.Fetch(name, false, progress)
		ch <- downloadDoneMsg{path: dest, err: err, gen: gen}
	}()
	return ch, func() tea.Msg { return <-ch } // first message; Update re-arms for the rest
}
```

The returned Cmd blocks on the channel, reads the first message, and returns it to `Update`. To keep polling, `Update` re-arms: when it sees a progress message with `gen == m.dlGen`, it returns `func() tea.Msg { return <-m.dlCh }`; the `downloadDoneMsg` case stops the loop.

- [ ] **Step 3: Wire the model routing**

- Add `dlCh chan tea.Msg` to `Model` (zero value nil).
- `downloadStartMsg` handler (model.go:387-395): `ch, cmd := startFetch(msg.name, m.dlGen); m.dlCh = ch`; keep status `"Downloading %s… (no progress info)"` (unchanged — no info yet); `return m, tea.Batch(cmd, m.spinner.Tick)`.
- `downloadProgressMsg` case (model.go:396-398): if `msg.gen != m.dlGen` → `return m, nil` (stale/dismissed); if `msg.known` → `m.dlProgress = clamp01(msg.frac)` and `m.status = fmt.Sprintf("Downloading %s…", m.dlName)` (drop the parenthetical once measured); else leave `dlProgress` to the spinner easing. Return `m, func() tea.Msg { return <-m.dlCh }` while the channel is still live.
- `downloadDoneMsg` case: `m.dlCh = nil` alongside the existing `m.dlActive = false`. The stale-gen branch (model.go:402-407) also sets `m.dlCh = nil`.
- Orphan behavior note: a dismissed fetch keeps downloading in a goroutine and its late sends go to a channel nobody reads (buffered 8, then the goroutine parks) — parity with today's orphaned `fetchCmd`; no leak-free guarantee required here.

- [ ] **Step 4: Update and add TUI tests**

In `model_test.go`:
- Update `TestDownload_ShowsProgress` (line 149): send `downloadProgressMsg{frac: 0.5, gen: m.dlGen, known: true}` and additionally assert `m.status` no longer contains `"no progress info"`.
- Add `TestDownload_StaleProgressIgnored`: with `gen: m.dlGen - 1`, `m.DownloadProgress()` stays 0 and status unchanged.
- `TestDownload_ShowsIndeterminateStatus` (line 224) keeps passing (start status still says "no progress info").
- Add `TestDownload_PollReArm`: after `downloadStartMsg`, assert the returned cmd is non-nil; then send `downloadProgressMsg{frac: 0.5, gen: m.dlGen, known: true}` and assert a non-nil re-arm cmd comes back. Never execute download cmds in tests — running one would hit the real network and break hermiticity.

Run: `go test ./internal/tui -run TestDownload -v` — green.

- [ ] **Step 5: Run the full suite**

Run: `go build ./... && go test ./... && go vet ./... && test -z "$(gofmt -l .)"`
Expected: all green.

- [ ] **Step 6: Commit**

```bash
git add internal/downloader internal/tui
git commit -m "feat(downloader): report measured download progress; honest indeterminate fallback"
```

---

### Task 9: Split `model.go` (pure move, zero behavior change)

**Files:**
- Modify: `internal/tui/model.go` (shrinks)
- Create: `internal/tui/update.go`, `internal/tui/download.go`, `internal/tui/import.go`
- Modify: `internal/tui/view.go` (receives `sizeWidgets`)

**Interfaces:**
- Produces: the same package-level names in the same package — no external signatures change; this task reorganizes files only. Subsequent tasks (none after) and the test suite are the contract.

- [ ] **Step 1: Move `Update`, `handleKey`, and `sizeWidgets`**

- `update.go`: `Update` + `handleKey` (key routing stays together).
- `view.go`: append `sizeWidgets` (layout sizing is view concern).

- [ ] **Step 2: Move download machinery into `download.go`**

`refreshDownloadItems`, `clampDownloadWindow`, `dismissDownload`, `startFetch`, and the download msg types (`downloadStartMsg`, `downloadProgressMsg`, `downloadDoneMsg`); `dlWindow` const; keep `Downloading()`/`DownloadProgress()` accessors wherever the struct lives.

- [ ] **Step 3: Move import machinery into `import.go`**

`importCmd`, `expandHome`, `completePath`, `longestCommonPrefix`, `importDoneMsg`, and the `overlayImportClash` handling helpers; keep the `overlay` enum + `FilterMsg` and the `Model` struct in `model.go` (they are the model surface).

- [ ] **Step 4: Verify the move is behavior-neutral**

Run: `go build ./... && go test ./... && go vet ./... && test -z "$(gofmt -l .)"`
Expected: identical green suite; `git diff --stat` shows only moved lines (no additions beyond file boundaries).

- [ ] **Step 5: Commit**

```bash
git add internal/tui
git commit -m "refactor(tui): split model.go into update/download/import files; layout sizing into view.go"
```

---

## Self-Review

- **Spec coverage:** q-always-quits (Go spec §4 "q quits (restores if dirty)"; Python parity) → Task 2. Commit toast (Python toast parity) → Task 3. Slot validity as a single error source (spec §2 paths) → Task 4. Import clash prompt / `~` / tab-complete (Python spec §3.4, §4) → Task 7. Atomic writes and honest download status (spec §5 error handling) → Tasks 6, 8. Catalog/prompt/glyph/theme divergences → Task 1 (user-approved). On-disk contract, reload-after-install, backup naming — untouched by design.
- **Step scan:** every step carries a signature, message shape, command, or test name; no "handle edge cases" placeholders; implementer bodies are the only free choices left.
- **Type consistency:** `paths.SlotNames`, `paths.ColorsPath`, `importer.ClashError{Dest}`, `ProgressFunc(downloaded, total int64)`, `downloadProgressMsg{frac, gen, known}`, `importCmd(path, clash)`, `expandHome`, `completePath`, `entrySummary`, `casefoldHits`, `overlayImportClash`, `pendingImportPath`, `dlCh` — each defined once and referenced consistently.
- **Review Focus:** all seven lines have an owning test (Task 2 → q; Task 3 → toast; Task 6 → symlink; Task 7 → missing path, clash, `~` under TERMUX_HOME; Task 8 → unknown-total indeterminate).
- **Proportion:** tasks carry shapes and tests, not program transcripts; the two feature tasks (7, 8) are the longest because their message routing is the part an implementer cannot invent.