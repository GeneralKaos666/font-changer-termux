# termux-fonts

Change your Termux terminal font with a friendly fullscreen picker.
Browse your fonts, try each one live, and keep the one you love — with
automatic backups, so you can always go back.

The picker dresses itself in your Termux colors and shows your real
shell prompt in every candidate font. What you see is what you get.

## Install

You need [Go](https://go.dev/dl/) 1.26 or newer, then run:

```bash
go install github.com/GeneralKaos666/font-changer-termux/cmd/termux-fonts@latest
```

This puts a `termux-fonts` command on your system. If your shell can't
find it afterwards, add Go's install folder to your `PATH`, or install
straight into Termux's own folder instead:

```bash
GOBIN=$PREFIX/bin go install github.com/GeneralKaos666/font-changer-termux/cmd/termux-fonts@latest
```

Want to poke around safely first? Point the tool at a throwaway folder
so your real setup stays untouched:

```bash
TERMUX_HOME=$PREFIX/tmp/fc-go-demo termux-fonts --list
```

## Usage

Open the picker:

```bash
termux-fonts
```

The top half previews the highlighted font: a sample alphabet, symbol
coverage, facts about the font (glyph count and version), your actual
shell prompt, and which terminal slot it would fill. The bottom half is
your font collection — fonts already in use carry a `● slot` badge.
The list is ready for arrow keys right away; press `Tab` to jump to the
search box.

Prefer the command line? These work too (great for scripts):

```bash
termux-fonts --list
termux-fonts --apply "JetBrainsMono.ttf" --slot regular
```

Termux has four font slots: `regular`, `bold`, `italic`, and
`bold-italic` (`regular` is the default). `--apply` finds the font by
name (exact match first, then case-insensitive), checks that the file
is a healthy font, saves a backup of whatever was there before,
installs the new font, and tells Termux to reload.

### Keys

| Key       | What it does                        |
| --------- | ----------------------------------- |
| Space/`p` | Try the highlighted font, live      |
| Enter     | Keep the font you're trying         |
| `s`       | Switch between the four font slots  |
| `i`       | Add one of your own font files      |
| `d`       | Download a Nerd Font                |
| Esc       | Go back / undo an untried preview   |
| `q`       | Quit (undoes a preview you didn't keep) |
| Tab       | Jump to the search box              |

Just looking around changes nothing — a font is only installed when you
preview it (Space/`p`) or keep it (Enter).

## Matching your style

When you open the picker, it reads your Termux color theme
(`~/.termux/colors.properties`) and styles itself to match. Set a theme
with [termux-colors](https://github.com/GeneralKaos666/color-changer-termux)
and this picker follows along. No theme file? It falls back to calm
built-in colors.

## Backups — you can't break anything

The first time each session that a font would replace something, the old
file is copied to `~/.termux/backups/` with the date and time in its
name (for example `font-2026-10-06-120500.ttf`). Trying more fonts reuses
that same backup instead of piling up copies. And if you quit while
trying a font you never kept, the original is put back automatically.

## First run

If your font collection is empty but Termux already has a font active,
the tool copies it into the collection as `Current.ttf` on startup, so
the picker is never blank. This happens for both the visual picker and
the command-line flags.

## A note on the old Python version

This tool started life as a Python program. That version is retired and
no longer maintained — this Go rewrite replaces it completely. Curious
archaeologists can still find it in this repo's history from before the
Go commits (`git log --all -- src/termux_fonts/`).
