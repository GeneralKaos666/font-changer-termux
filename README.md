# termux-fonts

A Bubble Tea TUI to change the Termux terminal font, with live preview,
backup-once preview/commit/restore, font import, Nerd Font download, and
CLI flags for scripting.

The picker dresses itself in your Termux palette and shows your real
shell prompt in every candidate font — what you see is what you get.

## Install

Requires Go >= 1.26.

```bash
go install github.com/GeneralKaos666/font-changer-termux/cmd/termux-fonts@latest
```

Make sure `$(go env GOPATH)/bin` is on your `PATH` (or
`GOBIN=$PREFIX/bin go install ...` to drop the binary straight into
Termux's `bin`).

Uses `~/.termux` for fonts and settings; point it elsewhere for testing
with `TERMUX_HOME`:

```bash
TERMUX_HOME=$PREFIX/tmp/fc-go-demo termux-fonts --list
```

## Usage

Launch the interactive picker:

```bash
termux-fonts
```

The preview pane (top) shows a sample block, Nerd/powerline coverage,
real font metadata (glyph count, UPM, version), your live shell prompt,
and slot + backup status. The font list (bottom) marks applied fonts
with a `● slot` badge. The list is focused on launch — arrows move
immediately, `Tab` reaches the filter box.

Non-interactive use (also handy for scripts):

```bash
termux-fonts --list
termux-fonts --apply "JetBrainsMono.ttf" --slot regular
```

`--slot` is one of `regular`, `bold`, `italic`, `bold-italic`
(default: `regular`). `--apply` matches the library by exact name first,
then case-insensitively (ambiguous collisions are rejected), validates
the font, backs up the previous slot file once, installs the new font,
and asks Termux to reload settings via `termux-reload-settings` when
available.

### Keys

| Key       | Action                              |
| --------- | ----------------------------------- |
| Space/`p` | Preview the selected font (live)    |
| Enter     | Keep (commit) the previewed font    |
| `s`       | Cycle font slot                     |
| `i`       | Import a font file into the library |
| `d`       | Download a Nerd Font                |
| Esc       | Back / restore original font        |
| `q`       | Quit (restores original if dirty)   |
| Tab       | Focus the filter box                |

Highlighting a font only updates the info pane — nothing is applied
until you preview (Space/`p`) or commit (Enter).

## Theming

On startup the TUI reads `~/.termux/colors.properties` (if present) and
themes itself: background/foreground for chrome text, accent from
`color12` (fallback `color4`), muted from `color8`. Set a palette with
[termux-colors](https://github.com/GeneralKaos666/color-changer-termux)
and this picker follows it.

## Backups

The first install or preview for a slot copies the previous slot file
to `~/.termux/backups/<slot>-<timestamp>.<ext>` (for example
`font-2026-10-06-120500.ttf`). Repeat previews reuse that same backup,
so one session never piles up duplicates. Quitting with an uncommitted
preview restores the original bytes.

## First-run seed

If `~/.termux/fonts/` is empty but `~/.termux/font.ttf` exists, startup
seeds the library by copying it to `~/.termux/fonts/Current.ttf`, so
the picker is never empty on first launch. Seeding runs before both the
TUI and the `--list` / `--apply` paths.

## Note: Python version discarded

An earlier Python/Textual implementation of this tool lived in this
repo's history (and briefly on a `python-main` branch, now deleted).
It is superseded by this Go port and will not be maintained. If you
need the old code, it remains reachable in history before the Go
commits (`git log --all -- src/termux_fonts/`).
