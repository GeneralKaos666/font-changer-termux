# Minor-Nits Follow-up Fixes Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Fix the five carried minor nits from the whole-branch review of `fix/review-findings` (merged to `main` @ `1e7cf41` on 2026-10-10). None block the merge; this plan closes them for a clean spec/code/test surface:

1. Spec §3.5 never literally names the new `ask` clash mode in the `ImportFile` signature line (and the older Go-port plan doc has the same gap).
2. `downloader.Fetch` doc comment overstates: "skipped and failed fetches report nothing" (mid-copy failures do emit partial totals).
3. Pre-existing: the non-dirty install toast names the slot *filename* ("Installed font.ttf → regular") instead of the font — the same display class the commit-toast fix (Task 3 of the previous plan) resolved for the dirty path.
4. The `"i"` keep-both clash-prompt alias has no dedicated test (only `"r"` is covered; `"1"` is covered by a separate test).
5. The previous plan doc (`2026-10-09-review-findings-fixes.md`) retains two stale prose lines (Task 3 step 3's `filepath.Base(target)` slip and the "still compiles against the current name" `slotOrder` wording).

**Architecture:** Three small ordered tasks, one commit each, every task ends with `go build ./... && go test ./... && go vet ./... && gofmt -l .` green:

- Task 1 — docs only: mode-set enumerations (spec §3.5 + port plan), the `Fetch` doc comment, and the two stale plan-doc lines.
- Task 2 — code + test: the install toast names the font (TDD).
- Task 3 — test only: the `"i"` keep-both alias.

**Tech Stack:** Go >= 1.26 (go.mod), Bubble Tea confined to `internal/tui` + `cmd` entrypoint. No new dependencies.

**Spec:** `docs/superpowers/specs/2026-10-06-go-bubbletea-port-design.md` (primary). The install-toast change (Task 2) must keep the toast shape consistent with the blessed commit toast `"Kept <font> → <slot> slot"`.

## Global Constraints

- `TERMUX_HOME` wins for every Termux config path; tests must `t.Setenv("TERMUX_HOME", t.TempDir())` and never touch the real `~/.termux`.
- Only `internal/tui` may import bubbletea/bubbles/lipgloss; `cmd/termux-fonts/main.go` may import bubbletea to start the program.
- Slots `regular|bold|italic|bold-italic` → `font.ttf | font-bold.ttf | font-italic.ttf | font-bold-italic.ttf`; unknown slot is an error, never a fallback. Slots and colors path are single-sourced in `internal/paths` (`SlotNames`, `ColorsPath()`).
- Import clash modes (single enumeration, code is truth): `ask|error|keep-both|replace` — `importer.go:69` switch and the unknown-mode error text (`"choose from ask, error, keep-both, replace"`).
- Tests are hermetic by construction: no real network, no real shell, no real `$HOME`; TUI tests stub `capturePromptLines`/`fetchFont` in `TestMain` and use the `noReload` PATH stub.
- Checks: `go build ./...`, `go test ./...`, `go vet ./...`, `gofmt -l .` (must print nothing).

---

### Task 1: Clarify clash-mode docs, the `Fetch` progress comment, and the stale plan prose (nits 1, 2, 5)

**Files:**
- Modify: `docs/superpowers/specs/2026-10-06-go-bubbletea-port-design.md`
- Modify: `docs/superpowers/plans/2026-10-06-go-bubbletea-port.md`
- Modify: `internal/downloader/downloader.go` (comment only)
- Modify: `docs/superpowers/plans/2026-10-09-review-findings-fixes.md` (two stale lines)

**Steps:**

- [ ] **Step 1:** Spec §3.5, `docs/superpowers/specs/2026-10-06-go-bubbletea-port-design.md:69` —

  ```
  5. **importer** — `ImportFile(src, clash)` with `error|keep-both|replace`;
  ```
  →
  ```
  5. **importer** — `ImportFile(src, clash)` with `ask|error|keep-both|replace`;
  ```
  (`ask` raises `*ClashError` so the caller can prompt, as the §3.5 prose below already describes.)

- [ ] **Step 2:** Same enumeration fix in the older port plan, `docs/superpowers/plans/2026-10-06-go-bubbletea-port.md:194` —

  `(`clash` ∈ `error|keep-both|replace`)` → `(`clash` ∈ `ask|error|keep-both|replace`)`

- [ ] **Step 3:** `internal/downloader/downloader.go:137-138` — the `Fetch` doc comment's last sentence. Current:

  ```go
  // totals during the copy (see ProgressFunc); skipped and failed fetches
  // report nothing.
  ```
  →
  ```go
  // totals during the copy (see ProgressFunc); skipped fetches report
  // nothing, and failures never report a terminal completion.
  ```
  (Honest wording: mid-copy failures do emit per-read partial totals; only a terminal `(total, total)` completion is never reported on failure.)

- [ ] **Step 4:** Stale prose in `docs/superpowers/plans/2026-10-09-review-findings-fixes.md`:

  - **Line 180** (Task 3 step 3) — the toast snippet used `filepath.Base(target)`, but the shipped code reads the preview's source. Replace the step-3 sentence with the shipped shape (and refreshed actions.go line refs, which shifted after the Task 9 split):

    ```
    In `doCommit` (actions.go:53-77), when dirty: read `src, slot := st.Preview.Src, st.Preview.Slot` BEFORE `apply.CommitPreview(m.state)` (which nils `Preview`), then `m.status = fmt.Sprintf("Kept %s → %s slot", filepath.Base(src), slot) + reloadHint()`. The non-dirty `InstallFont` branch keeps using `m.slot` (correct there).
    ```

  - **Line 149** (Task 3 "Consumes") — disambiguate the ordering note. Current: `slotOrder` (Task 4 renames this to `paths.SlotNames` — this task still compiles against the current name). →
    `slotOrder` (renamed to `paths.SlotNames` by Task 4 — Task 3 executes before Task 4, so it uses the pre-rename name).

- [ ] **Step 5:** Verify no other doc enumerates the clash mode set without `ask` (grep `error|keep-both|replace` in `docs/` — the only remaining hits should be the intentionally unchanged `keep-both / replace / cancel` prose descriptions of the prompt labels).

- [ ] **Step 6:** Run the full gate.

- [ ] **Step 7:** Commit:

  ```bash
  git add docs/superpowers/specs/2026-10-06-go-bubbletea-port-design.md docs/superpowers/plans/2026-10-06-go-bubbletea-port.md internal/downloader/downloader.go docs/superpowers/plans/2026-10-09-review-findings-fixes.md
  git commit -m "docs: name the ask clash mode; fix Fetch progress wording; correct stale plan prose"
  ```

---

### Task 2: Install toast names the font and slot (nit 3)

**Files:**
- Modify: `internal/tui/actions.go` (`doCommit` non-dirty branch)
- Modify: `internal/tui/model_test.go` (new test)

**Background:** `doCommit`'s non-dirty branch (actions.go:66-77) copies the selected library entry into the current slot and toasts `"Installed " + filepath.Base(target) + " → " + m.slot` — `target` is the slot *path* (`.../fonts/font.ttf`), so the toast shows the slot filename, not the font. The dirty branch (fixed previously) toasts `"Kept Hack.ttf → regular slot"`. Make the install toast match: `"Installed Hack.ttf → regular slot"`.

**Steps:**

- [ ] **Step 1 (RED):** Add `TestCommit_InstallToastNamesFontAndSlot` to `internal/tui/model_test.go` (mirroring the `TestCommit_ToastNamesCommittedSlot` helper usage: `useTermuxHome`, `noReload`, `seedLibrary`):

  ```go
  func TestCommit_InstallToastNamesFontAndSlot(t *testing.T) {
  	useTermuxHome(t)
  	noReload(t)
  	seedLibrary(t, "Hack.ttf")

  	m := NewModel()
  	m = updateModel(t, m, keyRunes("enter")) // no preview → non-dirty install path
  	if !strings.HasPrefix(m.status, "Installed ") {
  		t.Fatalf("status = %q, want it to start with %q", m.status, "Installed ")
  	}
  	if !strings.Contains(m.status, "Hack.ttf") || !strings.Contains(m.status, "regular slot") {
  		t.Fatalf("status = %q, want the font name and slot in the toast", m.status)
  	}
  }
  ```

  Run `go test ./internal/tui -run TestCommit -v` — RED: today's toast is `"Installed font.ttf → regular..."` (slot filename), so `Contains("Hack.ttf")` fails.

- [ ] **Step 2 (GREEN):** `internal/tui/actions.go` non-dirty branch — toast names the selected entry and mirrors the commit toast shape:

  ```go
  _, err := apply.InstallFont(e.Path, m.slot)
  if err != nil {
  	m.status = "Install failed: " + err.Error()
  	return
  }
  m.status = fmt.Sprintf("Installed %s → %s slot", e.Name, m.slot) + reloadHint()
  ```

  **Compile trap:** `InstallFont`'s return value was `target`; switching the toast to `e.Name` makes `target` unused — it must become `_, err := apply.InstallFont(...)`. No other branch references `target` (the dirty branch has its own `target := apply.CommitPreview(...)`).

- [ ] **Step 3:** Run the focused test (GREEN), then the full gate. Confirm no existing test asserted the old `"Installed … → …"` (no trailing `slot`) shape — grep `Installed` in `internal/` tests; the only prior occurrence is actions.go:76; the CLI's `--apply` output is separate and untouched.

- [ ] **Step 4:** Commit:

  ```bash
  git add internal/tui/actions.go internal/tui/model_test.go
  git commit -m "fix(tui): install toast names the font and slot, matching the commit toast"
  ```

---

### Task 3: Test the `"i"` keep-both clash-prompt alias (nit 4)

**Files:**
- Modify: `internal/tui/model_test.go` (new test)

**Background:** In the clash-overlay, keys `1`/`i` → keep-both, `2`/`r` → replace, `esc` → cancel (update.go). `TestImport_ClashPromptKeepBoth` covers `"1"`, `TestImport_ClashPromptLetterKeys` covers `"r"`; `"i"` is untested (it shares `update.go`'s switch arm with `"1"`, so risk is trivial — pin it anyway). Note `"i"` opens the import input at the *main* overlay — the alias applies only while the clash overlay is active, which is what this test pins.

**Steps:**

- [ ] **Step 1:** Add `TestImport_ClashPromptLetterIKeepsBoth` to `internal/tui/model_test.go` (mirror `TestImport_ClashPromptKeepBoth`'s setup/assertions but press `"i"`):

  ```go
  func TestImport_ClashPromptLetterIKeepsBoth(t *testing.T) {
  	useTermuxHome(t)
  	noReload(t)
  	original := fixtureBytes(t, "b.ttf")
  	writeLibraryFont(t, "newfont.ttf", original)
  	src := stageFont(t, "newfont.ttf")

  	m := NewModel()
  	m = openImport(t, m, src)
  	m = pressEnter(t, m)
  	m = pressKey(t, m, "i") // alias for 1 (keep both)
  	if !strings.HasPrefix(m.status, "Imported ") {
  		t.Fatalf("status after i = %q, want it to start with %q", m.status, "Imported ")
  	}
  	if m.overlay != overlayNone {
  		t.Fatalf("overlay = %v after resolution, want overlayNone", m.overlay)
  	}
  	kept, err := os.ReadFile(filepath.Join(paths.FontsDir(), "newfont-1.ttf"))
  	if err != nil {
  		t.Fatalf("i did not keep both (no newfont-1.ttf): %v", err)
  	}
  	if string(kept) != string(fixtureBytes(t, "a.ttf")) {
  		t.Fatal("i kept the wrong bytes")
  	}
  	if got, _ := os.ReadFile(filepath.Join(paths.FontsDir(), "newfont.ttf")); string(got) != string(original) {
  		t.Fatal("i must leave the pre-existing file untouched")
  	}
  }
  ```

  Run `go test ./internal/tui -run 'TestImport_ClashPrompt' -v` — should pass immediately with the `update.go` alias wiring already in place (this task pins behavior, not a fix).

- [ ] **Step 2:** Run the full gate.

- [ ] **Step 3:** Commit:

  ```bash
  git add internal/tui/model_test.go
  git commit -m "test(tui): cover the i (keep-both) clash-prompt alias"
  ```

---

## Completion

Once Tasks 1-3 are green (each committed), run the final gate once more and review the branch diff against this plan before any integration. Expected surface: 4 docs lines/paragraphs + 1 Go comment + 1 toast line (+ the `target`→`_` rename) and 2 new tests — no behavior change beyond the toast text.