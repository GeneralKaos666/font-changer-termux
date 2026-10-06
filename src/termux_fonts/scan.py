"""Scan the font library and read the active Termux font slots."""

from __future__ import annotations

from pathlib import Path

from fontTools.ttLib import TTFont

from termux_fonts import paths
from termux_fonts.models import FontEntry

_PATTERNS: tuple[str, ...] = ("*.ttf", "*.otf", "*.TTF", "*.OTF")


def _family_style(path: Path) -> tuple[str, str]:
    """Return ``(family, style)`` from the font's ``name`` table.

    Falls back to the filename stem / ``"Regular"`` when the font cannot be
    parsed or carries no usable names.
    """
    try:
        with TTFont(str(path), lazy=True) as font:
            names = font["name"]
            family = (
                names.getDebugName(16) or names.getDebugName(1) or path.stem
            )
            style = names.getDebugName(17) or names.getDebugName(2) or "Regular"
            return family, style
    except Exception:  # noqa: BLE001 - fallback covers any unreadable file
        return path.stem, "Regular"


def _entry_for(path: Path) -> FontEntry:
    stat = path.stat()
    family, style = _family_style(path)
    return FontEntry(
        name=path.name,
        path=path,
        size=stat.st_size,
        mtime=stat.st_mtime,
        family=family,
        style=style,
    )


def list_library() -> list[FontEntry]:
    """List all ``.ttf``/``.otf`` fonts in the library, sorted by name.

    Returns ``[]`` when the library directory does not exist or is empty.
    Sorting is case-insensitive.
    """
    directory = paths.fonts_dir()
    if not directory.is_dir():
        return []
    seen: set[Path] = set()
    entries: list[FontEntry] = []
    for pattern in _PATTERNS:
        for path in directory.glob(pattern):
            if path in seen or not path.is_file():
                continue
            seen.add(path)
            entries.append(_entry_for(path))
    entries.sort(key=lambda entry: entry.name.lower())
    return entries


def read_active() -> dict[str, Path | None]:
    """Return the active font file per slot, ``None`` for empty slots."""
    active: dict[str, Path | None] = {}
    for slot in paths.SLOT_FILES:
        slot_path = paths.font_slot_path(slot)
        active[slot] = slot_path if slot_path.is_file() else None
    return active
