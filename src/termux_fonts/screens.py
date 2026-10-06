"""Font picker screen with manual live preview.

Highlighting a font updates the info pane only and never touches the
active Termux font files. Side effects happen solely through explicit
actions: preview (Space/p), commit (Enter), restore (Esc), quit (q).
"""

from __future__ import annotations

from pathlib import Path

from textual import work
from textual.app import ComposeResult
from textual.containers import Horizontal, Vertical
from textual.screen import Screen
from textual.widgets import Footer, Input, Label, ListItem, ListView, Static

from termux_fonts import apply, scan
from termux_fonts.downloader import NERD_FONTS, fetch
from termux_fonts.importer import import_file
from termux_fonts.models import FontEntry

SLOTS: tuple[str, ...] = ("regular", "bold", "italic", "bold-italic")

SAMPLE_TEXT = "The quick brown fox jumps over 0123456789 AaBbCcDdEe"

EMPTY_HINT = "No fonts in library — press i to import, d to download."


class _FontItem(ListItem):
    """A list row that carries its :class:`FontEntry`."""

    def __init__(self, entry: FontEntry) -> None:
        super().__init__(Label(entry.name))
        self.entry = entry


class FontPickerScreen(Screen):
    """Browse fonts, preview manually, commit or restore."""

    BINDINGS = [
        ("space", "preview", "Preview"),
        ("p", "preview", "Preview"),
        ("enter", "commit", "Keep"),
        ("s", "cycle_slot", "Slot"),
        ("i", "import_font", "Import"),
        ("d", "download_font", "Download"),
        ("escape", "back_or_restore", "Back"),
    ]

    CSS = """
    #picker {
        height: 1fr;
    }
    #picker-list {
        width: 42%;
        border: solid green;
    }
    #picker-preview {
        width: 58%;
        padding: 0 1;
    }
    #filter {
        dock: top;
    }
    """

    def __init__(self) -> None:
        super().__init__()
        self.state: dict = apply.new_session_state()
        self.slot: str = "regular"
        self.entries: list[FontEntry] = []
        self.filtered: list[FontEntry] = []

    def compose(self) -> ComposeResult:
        yield Input(placeholder="Filter fonts...", id="filter")
        with Horizontal(id="picker"):
            yield ListView(id="picker-list")
            with Vertical(id="picker-preview"):
                yield Static(EMPTY_HINT, id="info")
                yield Static(SAMPLE_TEXT, id="sample")
                yield Static("Slot: regular", id="slot")
                yield Static("", id="status")
        yield Footer()

    def on_mount(self) -> None:
        self.entries = scan.list_library()
        self._show_entries(self.entries)

    # -- list / filter ----------------------------------------------------

    def _show_entries(self, entries: list[FontEntry]) -> None:
        """Replace list rows synchronously (mount-safe without a pilot)."""
        self.filtered = list(entries)
        list_view = self.query_one("#picker-list", ListView)
        list_view.clear()
        for entry in self.filtered:
            list_view.append(_FontItem(entry))
        if self.filtered:
            list_view.index = 0
        self._update_info()

    async def _refresh_list(self, query: str) -> None:
        query = query.casefold()
        matches = [
            e
            for e in self.entries
            if query in e.name.casefold() or query in e.family.casefold()
        ]
        list_view = self.query_one("#picker-list", ListView)
        await list_view.clear()
        self.filtered = matches
        for entry in matches:
            await list_view.append(_FontItem(entry))
        if matches:
            list_view.index = 0
        self._update_info()

    async def on_input_changed(self, event: Input.Changed) -> None:
        if event.input.id != "filter":
            return
        await self._refresh_list(event.value)

    def on_list_view_highlighted(self, event: ListView.Highlighted) -> None:
        # Info only — highlighting must never apply a font.
        self._update_info(getattr(event.item, "entry", None))

    def on_list_view_selected(self, event: ListView.Selected) -> None:
        # ListView consumes Enter as select; Enter means commit.
        self.action_commit()

    # -- info pane --------------------------------------------------------

    def _selected_entry(self) -> FontEntry | None:
        highlighted = self.query_one("#picker-list", ListView).highlighted_child
        entry = getattr(highlighted, "entry", None)
        if entry is not None:
            return entry
        return self.filtered[0] if self.filtered else None

    def _update_info(self, entry: FontEntry | None = None) -> None:
        entry = entry if entry is not None else self._selected_entry()
        info = self.query_one("#info", Static)
        if entry is None:
            info.update(EMPTY_HINT)
        else:
            info.update(f"{entry.name}\n{entry.family} {entry.style}\n{entry.size} bytes")

    def _set_status(self, message: str) -> None:
        self.query_one("#status", Static).update(message)

    def _with_reload_hint(self, message: str) -> str:
        if apply.last_reload_ok() is False:
            return f"{message} ({apply.MANUAL_RESTART_HINT})"
        return message

    # -- font actions -----------------------------------------------------

    def action_preview(self) -> None:
        entry = self._selected_entry()
        if entry is None:
            self.notify("No font selected", severity="warning")
            return
        try:
            apply.preview_font(entry.path, self.slot, self.state)
        except (ValueError, OSError) as exc:
            self.notify(str(exc), severity="error")
            return
        status = self._with_reload_hint(
            f"Preview {entry.name} — Enter keeps, Esc restores"
        )
        self._set_status(status)
        self.notify(status)

    def action_commit(self) -> None:
        if apply.is_preview_dirty(self.state):
            target = apply.commit_preview(self.state)
            self._set_status(f"Kept {target.name if target else ''}")
            self.notify("Preview kept")
            return
        entry = self._selected_entry()
        if entry is None:
            self.notify("No font selected", severity="warning")
            return
        try:
            target = apply.install_font(entry.path, self.slot)
        except (ValueError, OSError) as exc:
            self.notify(str(exc), severity="error")
            return
        status = self._with_reload_hint(f"Installed {target.name}")
        self._set_status(status)
        self.notify(f"Installed {entry.name}")

    def action_cycle_slot(self) -> None:
        index = SLOTS.index(self.slot)
        self.slot = SLOTS[(index + 1) % len(SLOTS)]
        self.query_one("#slot", Static).update(f"Slot: {self.slot}")

    def action_import_font(self) -> None:
        self.app.push_screen(ImportScreen(), self._after_library_changed)

    def action_download_font(self) -> None:
        self.app.push_screen(DownloadScreen(), self._after_library_changed)

    def _after_library_changed(self, result: Path | None) -> None:
        if result is None:
            return
        self.entries = scan.list_library()
        self._show_entries(self.entries)
        self._set_status(f"Added {result.name}")
        self.notify(f"Added {result.name}")

    def action_back_or_restore(self) -> None:
        if apply.is_preview_dirty(self.state):
            try:
                apply.restore_original(self.state)
            except OSError as exc:
                self.notify(str(exc), severity="error")
                return
            status = self._with_reload_hint("Preview discarded — original restored")
            self._set_status(status)
            self.notify("Original restored")

    def on_unmount(self) -> None:
        if apply.is_preview_dirty(self.state):
            try:
                apply.restore_original(self.state)
            except OSError:
                pass


class ImportScreen(Screen):
    """Ask for a font file path and import it into the library."""

    BINDINGS = [("escape", "cancel", "Cancel")]

    def compose(self) -> ComposeResult:
        yield Label("Import font file (full path):")
        yield Input(placeholder="/sdcard/Download/MyFont.ttf", id="import-path")
        yield Footer()

    def on_input_submitted(self, event: Input.Submitted) -> None:
        if event.input.id != "import-path":
            return
        try:
            dest = import_file(Path(event.value).expanduser())
        except (ValueError, OSError, FileExistsError) as exc:
            self.notify(str(exc), severity="error")
            return
        self.dismiss(dest)

    def action_cancel(self) -> None:
        self.dismiss(None)


class DownloadScreen(Screen):
    """Pick a Nerd Font by name and download it into the library."""

    BINDINGS = [("escape", "cancel", "Cancel")]

    def compose(self) -> ComposeResult:
        yield Label("Available Nerd Fonts (type a name):")
        yield Static("\n".join(sorted(NERD_FONTS)), id="dl-list")
        yield Input(placeholder="JetBrainsMono-Regular", id="download-name")
        yield Static("", id="dl-status")
        yield Footer()

    def on_input_submitted(self, event: Input.Submitted) -> None:
        if event.input.id != "download-name":
            return
        name = event.value.strip()
        if not name:
            self.notify("Enter a font name", severity="warning")
            return
        # Non-blocking: fetch() does network I/O, so run it in a worker
        # thread and keep the TUI responsive.
        self.query_one("#dl-status", Static).update(
            f"Downloading {name}... (UI stays responsive)"
        )
        event.input.disabled = True
        self._download(name)

    @work(thread=True, exclusive=True)
    def _download(self, name: str) -> None:
        try:
            dest = fetch(name)
        except (ValueError, OSError) as exc:
            self.app.call_from_thread(self._download_failed, str(exc))
            return
        except Exception as exc:  # noqa: BLE001 - worker must never die silently
            self.app.call_from_thread(self._download_failed, f"{exc}")
            return
        self.app.call_from_thread(self._download_done, dest)

    def _download_failed(self, message: str) -> None:
        try:
            self.query_one("#download-name", Input).disabled = False
        except Exception:  # noqa: BLE001 - screen may be gone; notify anyway
            pass
        try:
            self.query_one("#dl-status", Static).update(
                f"Download failed: {message}"
            )
        except Exception:  # noqa: BLE001 - screen may be gone; notify anyway
            pass
        self.notify(message or "Download failed", severity="error")

    def _download_done(self, dest: Path) -> None:
        try:
            self.dismiss(dest)
        except Exception:  # noqa: BLE001 - screen already closed (Esc)
            self.notify(f"Downloaded {dest.name} (screen already closed)")

    def action_cancel(self) -> None:
        self.dismiss(None)
