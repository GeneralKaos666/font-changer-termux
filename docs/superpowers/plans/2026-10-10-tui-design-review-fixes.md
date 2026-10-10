# TUI Design-Review Fixes Implementation Plan

**Goal:** Close every finding from the `tui-design` skill review of the
existing Bubble Tea picker (`main` @ `5229111`, tag `v0.2.0`). The review
grouped findings by user harm; this plan fixes all of them plus the minor
nits, with a test per behavior.

**Findings and fixes:**

1. **No alternate screen** — quitting erased only the last line, leaving a
   full frame in scrollback. Fix: `tea.WithAltScreen()`.
2. **`View` hit the disk every frame** — `PreviewPane` called
   `scan.Describe` (file read + `sfnt.Parse`) on each render, ~10fps during
   downloads. Fix: cache the selected entry's metadata in `Model`, refreshed
   by `syncDetail` whenever the selection moves (never from `View`).
3. **Ctrl+C was a no-op** — the list's quit keybindings are disabled and the
   app bound only `q`. Fix: Ctrl+C quits from any screen, restoring an
   uncommitted preview; Ctrl+Z suspends and the library is re-read on resume.
4. **No width floor** — the 97-cell footer wrapped at 80 columns, the
   two-column join needed >= 48, and there was no "too small" state. Fix:
   width-aware (ANSI-safe) line clamping; a single-pane fold below 80
   columns (preview hidden, list full-width); a centered notice below 44x8.
5. **No non-TTY guard** — a piped run emitted a frame and hung (no
   `WindowSizeMsg`, no input). Fix: detect a non-character-device stdin/stdout
   before launching the picker and exit 2 with a hint.
6. **Slow first frame** — `NewModel` ran the shell-prompt capture
   synchronously (up to 3s). Fix: capture in the `Init` command via
   `promptMsg`; the mock prompt renders until it lands.
7. **Selection invisible in monochrome** — termenv's `Ascii` profile strips
   *all* styling, so the accent-background selection vanished under
   `NO_COLOR`. Fix: a `▶` cursor marker on the selected row (color preserved
   when available).
8. **Static, redundant footer** — one 8-key bar plus a status line that
   repeated the keys. Fix: a context-sensitive help bar (main / filter /
   import / download / clash) and a launch status that no longer lists keys.

**Minor nits:**

- `-h`/`--help` exited 2; now returns 0 (usage is a successful request).
- No ASCII fallback: `--ascii` / `NERDFONT_CHANGER_ASCII=1` swaps chrome
  (`+-| * > ...`) while keeping the Unicode glyph samples.
- A dismissed download parked its producer goroutine; `releaseFetch` now
  drains the orphaned channel so it can finish and exit.

**Architecture:** `internal/tui` (the only bubbletea importer besides the
`cmd` entrypoint) plus the `cmd/nerdfont-changer/main.go` launch path.
No behavior change to the on-disk contract or the apply/import/download
packages.

**Tech Stack:** Go >= 1.26; new direct dependency: `github.com/charmbracelet/x/ansi`
(already present transitively) for ANSI-aware truncation.

**Spec:** `docs/superpowers/specs/2026-10-06-go-bubbletea-port-design.md`
(sections 2, 3.7 and 8 updated to match).

## Global Constraints

- `TERMUX_HOME` wins for every Termux path; tests use
  `t.Setenv("TERMUX_HOME", t.TempDir())` and the `noReload` PATH stub, and
  never touch the real `~/.termux`, the network, or a real shell.
- Slots `regular|bold|italic|bold-italic` unchanged; unknown slot stays an
  error. Exit codes unchanged: 0 ok, 1 runtime error, 2 usage/ambiguous.
- Checks per task: `go build ./...`, `go test ./...`, `go vet ./...`,
  `gofmt -l .` (prints nothing).

## Tasks (all complete on this branch)

- **Alt screen + non-TTY guard + `-h` exit 0 + `--ascii`** — `main.go`,
  `main_test.go`.
- **Ctrl+C / Ctrl+Z / resume** — `update.go` (`quit`, `ctrl+z`,
  `tea.ResumeMsg`), tests.
- **Async prompt capture** — `model.go` (`promptMsg`, `Init`), `update.go`,
  `model_test.go`.
- **Cached `Describe`** — `model.go` (`syncDetail`, `detail*` fields),
  `view.go`, `model_test.go`.
- **Width floor + single pane + context footer** — `view.go`
  (`sizeWidgets`, `View`, `clampLine`, `tooSmallView`, `helpBar`), tests.
- **Monochrome selection marker + ASCII chrome** — `view.go`
  (`fontDelegate.Render`, `markerGlyph`, `borderFor`, `ellipsis`,
  `badgeDot`, `SetASCII`), `model.go` (`appliedBadges`), tests.
- **Download channel drain** — `download.go` (`releaseFetch`), test.

## Completion

`go build ./...`, `go test ./...`, `go vet ./...` and `gofmt -l .` are
green; the CLI was sanity-checked by hand (`--version` 0, `-h` 0,
`--list` 0, piped interactive 2). No files outside `internal/tui`,
`cmd/nerdfont-changer`, `go.mod`/`go.sum`, README and this docs set changed.
