# Font Changer for Termux — Design Spec (Hybrid, TUI-first)

Date: 2026-10-04
Status: draft, awaiting user review
Related: `color-changer-termux` (`termux-colors`), `~/.termux/fonts.sh`, `fontmerger`

## 1. Intent & success criteria

**User intent:** full manager for Termux terminal fonts, same feel as
`termux-colors` TUI, with a path to a real Android APK later.

**Phase 1 (this spec):** Python TUI run inside Termux as `termux-fonts`.
Browse/apply fonts from local library, import custom files, optionally
download Nerd Fonts, manage all 4 Termux slots, safe backups, instant
reload, manual live preview (Space/p applies highlighted font to the
terminal itself so the TUI re-renders in it).

**Phase 2 (deferred):** thin Android APK wrapper (tap icon) reusing Phase 1
core via Termux API / widget shortcut. Not designed in detail here.

**Success =**
- `termux-fonts` lists all `~/.termux/fonts/*.ttf|*.otf` (22 files observed
  on author's device) + shows active `font*.ttf`.
- User can apply Regular/Bold/Italic/Bold-Italic with backup prompt, and
  Termux font changes instantly via `termux-reload-settings`.
- Import + Nerd-Font download work offline-tolerant (local-only mode works
  with no network).
- Tests never touch real `~/.termux` (via `TERMUX_HOME` override).
- Core file ops are UI-free pure functions, reusable from a future APK.

**Non-goals (Phase 1):** in-pane raster of uninstalled TTFs (Textual
cannot raster TTFs; live preview is done by applying to the terminal),
OTF→TTF conversion
(defer to `fontmerger`), APK build/signing, system-wide Android font
replacement.

## 2. Architecture (approved)

Single Python package `termux-fonts`, mirroring `color-changer-termux`:

```
font-changer-termux/
  pyproject.toml          # name=termux-fonts, deps: textual>=0.60, fontTools (optional but preferred)
  src/termux_fonts/
    __init__.py
    paths.py              # TERMUX_HOME-aware: home(), fonts_dir(), font_slot(), backups_dir()
    models.py             # FontEntry(name, path, size, mtime, slots)
    scan.py               # list_library(), read_active()
    validate.py           # magic-byte + TTFont open check
    apply.py              # backup_if_exists(), install_font(), reload()
    importer.py           # import_file()
    downloader.py         # NERD_FONTS table (URLs from fonts.sh v3.2.1), fetch()
    app.py                # Textual App: main() -> termux-fonts
  tests/
    test_scan.py, test_apply.py, test_import.py, test_backup.py
```

UI-free core (`paths/scan/validate/apply/importer/downloader`) has zero
Textual imports. `app.py` is a thin shell. This lets Phase 2 call the same
core through `termux-api` RUN_COMMAND or a widget script with no rewrite.

`TERMUX_HOME` env (same as `termux-colors`): when set, all paths resolve
under it. Tests and manual dry-runs use a temp dir.

## 3. Components (approved)

1. **Library scanner** — glob `fonts_dir/*.ttf|*.otf` (case-insensitive),
   sort by name. For each: size, mtime, optional `fontTools.TTFont`
   family/style query (fallback to filename if fontTools missing).
   Also `read_active()`: for each slot in
   `[font.ttf, font-bold.ttf, font-italic.ttf, font-bold-italic.ttf]`
   report exists/missing + size.

2. **Preview pane + manual live preview** — highlight updates info only
   (no auto-apply). Shows file name, family/style, size, mtime, slots,
   plus sample block. `Space`/`p` live-previews: copies highlighted font
   to the selected slot + `termux-reload-settings`, so the whole TUI
   re-renders in that font. `Esc` after uncommitted preview restores
   session-original + reloads.

3. **Applier** — `install_font(src, slot)` + `preview_font(src, slot)`
   + `restore_original()`:
   `slot` ∈ {regular, bold, italic, bold-italic} → target
   `font.ttf | font-bold.ttf | font-italic.ttf | font-bold-italic.ttf`.
   Session start snapshots original `font*.ttf` hashes (no copy yet).
   First preview/commit needing overwrite creates one timestamped backup
   (`font-YYYY-MM-DD-HHMMSS.ttf` under `~/.termux/backups/`, mirroring
   `termux-colors`). `preview_font` copies + `termux-reload-settings`
   (via `shutil.which`, `subprocess.run` check, no shell) and marks
   preview-dirty. `Enter` commits (clears dirty, keeps font, no restore).
   `Esc` with dirty preview restores snapshot + reloads.

4. **Importer** — `import_file(path)`:
   validate → copy into `fonts_dir/` (prompt on name clash:
   Keep-both with `-N` suffix / Replace-with-backup / Cancel).
   Handles `/sdcard` EACCES with `termux-setup-storage` hint.

5. **Downloader** — table from `fonts.sh` (`NF_VERSION=v3.2.1`):
   JetBrainsMono Light/Regular, Hack, FiraCode, SourceCodePro, IosevkaTerm,
   Mononoki, Terminus, CascadiaCode, BlexMono, AnonymicePro, MesloLGS NF.
   `fetch(name)`: skip if same-size file exists unless `--force`,
   download via `urllib` (stdlib, no curl dep), rescan after.

## 4. Data flow (approved)

- Browse: filter box (substring, like `termux-colors`) → list →
  highlight updates info + sample pane (no font change).
- Live preview (manual): `Space`/`p` → validate → backup-on-first-preview
  if target exists → copy highlighted to selected slot →
  `termux-reload-settings` → status `Preview X in font.ttf — Enter keeps,
  Esc restores`. Re-press on another row re-previews. Slot key `s`
  switches target slot for subsequent previews/commits.
- Commit: `Enter` → slot confirm (defaults to current preview slot) →
  if already previewing same file+slot, just clear dirty + toast
  `Kept X → font.ttf`; else copy + reload. No extra backup (already done
  on first preview).
- Import: `i` → path input (tab-complete $HOME) → validate →
  clash prompt → copy → rescan, highlight new entry.
- Download: `d` → Nerd list with cached/uncached badge → `Enter` fetches
  with progress → rescan.
- Quit: `Esc` with dirty preview restores session-original + reloads,
  then backs out; `Esc` clean backs out; `q` from main quits (restores
  first if dirty, unless already committed). All overwrites backed up;
  `Esc` never loses original.

Paths always via `paths.py`, so `TERMUX_HOME=/tmp/xyz pytest` exercises
real copy/backup/reload-mocked flows.

## 5. Error handling (approved)

- Corrupt/invalid: magic check (`00 01 00 00`, `OTTO`, `true`, `typ1`)
  + `TTFont` open attempt (if fontTools present). Reject before any copy,
  show reason, keep current font untouched.
- Overwrite: never silent and never twice. First preview/commit needing
  overwrite creates one backup; further previews in same session reuse
  it. Clash prompt for library imports.
- Download fail: keep old state, show HTTP/curl-equivalent error,
  partial file removed.
- Missing `termux-reload-settings` (non-Termux run): warn + print manual
  step (`restart Termux app`), still leave copied font in place.
- `/sdcard` EACCES: suggest `termux-setup-storage`, offer copy-to-`~/`
  first workaround.
- OTF input: allowed as library entry (Termux accepts OTF in `font.ttf`
  slot on modern builds); no auto-convert in Phase 1, point to
  `fontmerger` in help text if merge/convert needed.

## 6. Testing & scope (approved)

- `pytest` mirrors `color-changer`: temp `TERMUX_HOME`, fixtures for fake
  fonts dir + active slots; cases: scan sorting, apply+backup naming,
  preview-dirty → commit keeps / Esc restores, import clash,
  invalid-file reject, downloader skip-if-exists (mocked
  HTTP), reload invoked (mock `subprocess.run`).
- Manual: on nubia aarch64, Termux 2.0.0, `pip install -e .` →
  `termux-fonts`, apply each slot, `termux-reload-settings` observed,
  restart persistence check.
- Requires: Python ≥3.10, `pkg install python`, `pip install .`
  (`textual>=0.60`, `fontTools` recommended).
- Out of scope Phase 1: APK, in-pane raster of uninstalled fonts, font
  merging, system fonts, root tricks.

## 7. Phase 2 notes (not specced)

Options preserved: (a) `termux-widget` shortcut launching `termux-fonts`;
(b) minimal APK firing `com.termux.app.RUN_COMMAND` to a core CLI
(`termux-fonts --apply <name> --slot regular` — hence core must stay
CLI-callable); (c) full Compose Material3 manager with SAF export/import
until Termux exposes a font API. No Phase 2 code in Phase 1.

## 8. Open questions for review

- Command name `termux-fonts` final? (matches `termux-colors`).
- Ship a small builtin (e.g. copy current `font.ttf` into library if
  library empty), like `termux-colors` builtin themes? Proposed: yes.
- Include `fontTools` as hard dep or optional? Proposed: hard dep
  (pure-Python, Termux-safe, needed for validation).
