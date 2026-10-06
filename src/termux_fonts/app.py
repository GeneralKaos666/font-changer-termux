"""Textual application entry point for termux-fonts."""

from __future__ import annotations

import argparse
import shutil
import sys
from pathlib import Path

from textual.app import App

from termux_fonts import apply, paths, scan
from termux_fonts.screens import FontPickerScreen


class TermuxFontsApp(App):
    """Browse fonts with manual live preview, commit, and restore."""

    TITLE = "Termux Fonts"

    BINDINGS = [
        ("space", "preview", "Preview"),
        ("p", "preview", "Preview"),
        ("enter", "commit", "Keep"),
        ("s", "cycle_slot", "Slot"),
        ("i", "import_font", "Import"),
        ("d", "download_font", "Download"),
        ("escape", "back_or_restore", "Back"),
        ("q", "quit_app", "Quit"),
    ]

    def on_mount(self) -> None:
        self.push_screen(FontPickerScreen())

    def _picker(self) -> FontPickerScreen | None:
        screen = self.screen
        return screen if isinstance(screen, FontPickerScreen) else None

    def _forward(self, action: str) -> None:
        picker = self._picker()
        if picker is not None:
            getattr(picker, f"action_{action}")()

    def action_preview(self) -> None:
        self._forward("preview")

    def action_commit(self) -> None:
        self._forward("commit")

    def action_cycle_slot(self) -> None:
        self._forward("cycle_slot")

    def action_import_font(self) -> None:
        self._forward("import_font")

    def action_download_font(self) -> None:
        self._forward("download_font")

    def action_back_or_restore(self) -> None:
        self._forward("back_or_restore")

    def action_quit_app(self) -> None:
        picker = self._picker()
        if picker is not None and apply.is_preview_dirty(picker.state):
            try:
                apply.restore_original(picker.state)
            except OSError:
                pass
        self.exit()


def ensure_builtin_seed() -> Path | None:
    """Seed the library from the active ``font.ttf`` when it is empty.

    When ``fonts/`` holds no fonts but the ``regular`` slot file exists,
    copy it to ``fonts/Current.ttf`` so first launch is never empty.
    Returns the seed path, or ``None`` when no seeding was needed.
    """
    if scan.list_library():
        return None
    src = paths.font_slot_path("regular")
    if not src.is_file():
        return None
    dest = paths.fonts_dir() / "Current.ttf"
    if dest.is_file():
        return dest
    dest.parent.mkdir(parents=True, exist_ok=True)
    shutil.copy2(src, dest)
    return dest


def _build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(
        prog="termux-fonts",
        description="Change the Termux terminal font (TUI or CLI).",
    )
    parser.add_argument(
        "--list",
        action="store_true",
        help="Print library font names and exit.",
    )
    parser.add_argument(
        "--apply",
        metavar="NAME",
        default=None,
        help="Install library font NAME into --slot and exit.",
    )
    parser.add_argument(
        "--slot",
        default="regular",
        choices=sorted(paths.SLOT_FILES),
        help="Font slot for --apply (default: regular).",
    )
    return parser


def main(argv: list[str] | None = None) -> None:
    """Run the font picker TUI, or handle --list / --apply for CLI use."""
    ensure_builtin_seed()
    parser = _build_parser()
    args = parser.parse_args(argv)
    entries = scan.list_library()
    if args.list:
        for entry in entries:
            print(entry.name)
        return
    if args.apply is not None:
        exact = [e for e in entries if e.name == args.apply]
        if len(exact) == 1:
            match = exact[0]
        else:
            lowered = args.apply.casefold()
            ci = [e for e in entries if e.name.casefold() == lowered]
            if not ci:
                parser.error(f"unknown font: {args.apply!r}")
            if len(ci) > 1:
                names = ", ".join(e.name for e in ci)
                parser.error(f"ambiguous font {args.apply!r}; matches: {names}")
            match = ci[0]
        try:
            target = apply.install_font(match.path, args.slot)
        except (ValueError, OSError) as exc:
            print(f"error: {exc}", file=sys.stderr)
            raise SystemExit(1) from exc
        if apply.last_reload_ok() is False:
            print(f"warning: {apply.MANUAL_RESTART_HINT}", file=sys.stderr)
        print(f"Applied {match.name} to {args.slot} ({target})")
        return
    TermuxFontsApp().run()


if __name__ == "__main__":
    main()
