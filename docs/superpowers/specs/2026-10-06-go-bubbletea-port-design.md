# Font Changer Go Port (Bubble Tea) — Design Spec

Date: 2026-10-06
Status: draft, awaiting user review
Related: `docs/superpowers/specs/2026-10-04-font-changer-design.md` (Python behavior authority),
`docs/superpowers/plans/2026-10-04-font-changer.md`
Motivation: learning project (Go + Elm architecture); single static binary, no Python/pip deps.

## 1. Intent & success criteria

**Intent:** a git branch `go-bubbletea` forked from `main` (02dcd25) carrying a
full Go rewrite of `termux-fonts`, behavior-compatible with the Python TUI.

**Success =**
- `go build ./...` on-device (Go 1.27.1 android/arm64) in seconds; `go test ./...` green.
- Single `termux-fonts` binary: browse 22-font library, filter, manual live
  preview (Space/p), commit (Enter), restore (Esc), 4 slots, import, Nerd download.
- Same on-disk contract: `~/.termux/fonts/`, `font*.ttf` slots,
  `~/.termux/backups/<slot>-YYYY-MM-DD-HHMMSS.ttf` (one per slot, anchored reuse — filename-scoped, Python parity),
  `termux-reload-settings` after every install/preview/restore.
- `TERMUX_HOME` override honored so tests never touch real `~/.termux`.
- Lessons carried over: list focused on launch (arrows work, no Tab needed);
  per-slot anchored backup glob (cross-slot collision fix); Enter single-commit;
  OSError-tolerant actions; non-blocking downloads.

**Non-goals:** changing the on-disk contract; APK wrapper; Ratatui variant;
feature parity beyond the Python TUI — except deliberate Go-only additions:
the full 46-family Nerd Font catalog with the two-column filtered download
picker; the live shell-prompt row in the preview pane (captures `$SHELL -ic`
once per session, ANSI-safe, mock fallback); and the glyph/UPM/version
metadata line.

## 2. Architecture (approved)

Branch `go-bubbletea`, Go module at repo root on that branch:

```
go.mod                            # module github.com/GeneralKaos666/font-changer-termux, go >= 1.26
cmd/termux-fonts/main.go          # argparse-equivalent flags + TUI entry
internal/paths/paths.go           # TERMUX_HOME-aware dirs, SLOT_FILES
internal/scan/scan.go             # list_library, read_active, FontEntry
internal/validate/validate.go     # magic + sfnt parse check
internal/apply/apply.go           # backup-once, preview/commit/restore, reload
internal/importer/importer.go     # validate+copy, clash policy
internal/downloader/downloader.go # 46-family Nerd catalog (one Regular per family plus JetBrainsMono-Light), skip-if-size, atomic rename
internal/tui/model.go             # Bubble Tea Model/Update/View + Bubbles widgets
```

Python sources remain on `main` only; the branch is a clean rewrite. The Python
spec stays the behavioral authority; conflicts resolve against it.

Elm pattern: one `Model` (entries, filter, slot, session state, status),
`Update` routes key/filter/download messages, `View` renders list (42%) +
preview (58%) via Bubbles `list` + `textinput` + `viewport`, styled with Lip Gloss.
The two-column split applies at >= 80 columns; narrower terminals fold the
preview away (single-pane list), and below 44x8 `View` shows a "terminal
too small" notice. The picker runs on the alternate screen (restored on
exit), so it never scribbles over scrollback.

## 3. Components (approved)

1. **paths** — `TermuxDir()`, `FontsDir()`, `FontSlotPath(slot)`,
   `BackupsDir()`, `SLOT_FILES`; `TERMUX_HOME` wins, else `$HOME/.termux`.
2. **scan** — glob `*.ttf|*.otf` (case variants), sort case-insensitive;
   family/style from `golang.org/x/image/font/sfnt` name table, filename
   fallback; `scan.Describe` drives the glyph/UPM/version line.
3. **validate** — magic `00 01 00 00 | OTTO | true | typ1` + `sfnt.Parse`
   (forces table read so magic+zeros fail, mirroring the `font["name"]` probe).
4. **apply** — `SnapshotOriginals`, `NewSessionState`, `EnsureBackupOnce`
   (anchored per-slot reuse: `{stem}-YYYY-MM-DD-HHMMSS{suffix}` full-match),
   `InstallFont`, `PreviewFont` (dirty+preview), `CommitPreview`, `RestoreOriginal`,
   `ReloadSettings` (`exec.LookPath` + run, missing → false + manual-restart hint).
5. **importer** — `ImportFile(src, clash)` with `ask|error|keep-both|replace`;
   EACCES → `termux-setup-storage` hint. The import flow gains a
   three-choice clash prompt (keep-both / replace / cancel), `~` expansion,
   and tab-complete of `$HOME` paths (deferred in the Python TUI, approved
   here).
6. **downloader** — full 46-family `NERD_FONTS` (one Regular per family
   plus `JetBrainsMono-Light`), pinned to the v3.2.1 `nfBase`; `Fetch(name,
   force)` via `net/http` HEAD size skip, temp + atomic rename, partial
   cleanup.
7. **tui** — Model/Update/View; keymap space/p/enter/s/i/d/esc/q identical to
   Python, plus Go-only ergonomics: Ctrl+C quits from any screen (restoring an
   uncommitted preview) and Ctrl+Z suspends (the library is re-read on resume);
   downloads as `tea.Cmd` producing messages; all I/O errors surface
   in the status line, never crash.
8. **theme** — `colors.properties` palette → adaptive accent/muted.

## 8. Visual style (approved)

Keyboard-first Lip Gloss treatment, no mouse required:
- Gradient title bar (`termux-fonts` + active slot), rounded borders with a
  single adaptive accent color that stays readable on dark and light terminals.
- Preview pane: large `AaBbCc 0123456789` sample block, Nerd/powerline
  coverage row (`  `), plus file info (family/style/size), the
  glyph/UPM/version info line, the live shell-prompt line, and slot +
  backup-status line (`font.ttf ← Hack • backup taken`).
- Filter input with match highlighting; a context-sensitive help bar that
  lists only the keys active on the current screen (main / filter / import /
  download / clash); spinner + progress bar on downloads; the selected row
  carries both an accent background and a `▶` cursor marker, so the selection
  survives monochrome terminals (where termenv strips all color and
  attributes).
- Width discipline: every rendered line is clamped to the terminal width, so
  the frame never wraps; `< 80` columns folds the preview away and `< 44x8`
  shows a floor notice naming the minimum size. The picker runs on the
  alternate screen, so it never scribbles over scrollback.
- Optional plain-ASCII chrome (`--ascii`, `NERDFONT_CHANGER_ASCII=1`) swaps
  the rounded borders, `●` badge, `▶` marker and `…` ellipsis for
  `+ - | * > ...`; the previewed glyph samples stay Unicode (they are the
  point of the tool).
- Restrained palette: accent + muted + default foreground only — no rainbow.

## 4. Data flow (approved)

List focused on launch; typing filters; Space/p → validate → backup-once →
copy → reload → status `Preview X — Enter keeps, Esc restores`; Enter commits
(clears dirty); Esc restores original + reloads; `s` cycles slots; `i`/`d`
overlays; `q` quits (restores if dirty). No auto-apply on highlight, same as Python.

## 5. Error handling (approved)

Invalid/corrupt rejected pre-copy; one backup per slot, anchored reuse (filename-scoped); reload-missing
shows manual-restart hint; `/sdcard` EACCES suggests `termux-setup-storage`;
TUI converts all I/O errors to status messages.

## 6. Testing (approved)

`go test ./...` mirrors the Python suite using `t.TempDir()` + `t.Setenv("TERMUX_HOME", ...)`:
scan sort, active-missing, invalid + corrupt-magic reject, empty library,
backup-once + multi-slot independence, preview-dirty commit/restore,
import clash/invalid, download skip-if-size via `httptest.Server`,
reload-missing returns false. Manual: build + run on nubia aarch64,
apply each slot, restart persistence.

## 7. Open questions

- Branch name `go-bubbletea` final? Proposed: yes.
- Keep Python `termux-fonts` script name for the Go binary too? Proposed: yes
  (`cmd/termux-fonts` builds a `termux-fonts` binary; install via `go install`).
- `golang.org/x/image` (sfnt) as only external dep besides bubbletea/bubbles/lipgloss?
  Proposed: yes; stdlib for everything else (net/http, os, exec, testing).
