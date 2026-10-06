# termux-fonts

A Textual TUI to change the Termux terminal font, with backup-once
preview/commit/restore, font import, Nerd Font download, and CLI flags
for scripting.

## Install

```bash
pip install .
```

This provides the `termux-fonts` command and the
`python -m termux_fonts` / `python -m termux_fonts.app` entry points.
Requires Python >= 3.10. Uses `~/.termux` for fonts and settings;
point it elsewhere for testing with `TERMUX_HOME`:

```bash
TERMUX_HOME=/tmp/fc-demo python -m termux_fonts.app --list
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
(default: `regular`). `--apply` validates the font, backs up the
previous slot file once, installs the new font, and asks Termux to
reload settings via `termux-reload-settings` when available.

### Keys

| Key    | Action                              |
| ------ | ----------------------------------- |
| Space/`p` | Preview the selected font (live) |
| Enter  | Keep (commit) the previewed font    |
| `s`    | Cycle font slot                     |
| `i`    | Import a font file into the library |
| `d`    | Download a Nerd Font                |
| Esc    | Back / restore original font        |
| `q`    | Quit (restores original if dirty)   |
| type   | Filter the font list                |

## Backups

The first install or preview for a slot copies the previous slot file
to `~/.termux/backups/<slot>-<timestamp>.<ext>` (for example
`font-2026-10-06-120500.ttf`). Repeat previews reuse that same backup,
so one session never piles up duplicates. Quitting with an uncommitted
preview restores the original bytes.

## First-run seed

If `~/.termux/fonts/` is empty but `~/.termux/font.ttf` exists, startup
seeds the library by copying it to `~/.termux/fonts/Current.ttf`, so
the picker is never empty on first launch.

## Phase-2 note

Planned follow-ups: multi-slot preview in one pass, font removal from
the library, and richer preview rendering. The `--list` / `--apply`
flags already expose the core for reuse by those features and by
external scripts.
