"""Textual application entry point for termux-fonts."""

from __future__ import annotations

from textual.app import App

from termux_fonts import apply
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
            apply.restore_original(picker.state)
        self.exit()


def main() -> None:
    """Run the font picker TUI."""
    TermuxFontsApp().run()


if __name__ == "__main__":
    main()
