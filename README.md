# termux-fonts (Go / Bubble Tea port)

A Bubble Tea TUI to change the Termux terminal font, with backup-once
preview/commit/restore, font import, Nerd Font download, and CLI flags
for scripting.

## Build

```bash
go build ./...
go build -o /tmp/termux-fonts-go ./cmd/termux-fonts
```

Requires Go >= 1.26 (pinned via `golang.org/x/image v0.46.0` and the
Bubble Tea stack). Uses `~/.termux` for fonts and settings; point it
elsewhere for testing with `TERMUX_HOME`:

```bash
TERMUX_HOME=/tmp/fc-go-demo /tmp/termux-fonts-go --list
```

## Usage

Launch the interactive picker:

```bash
termux-fonts
```

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
