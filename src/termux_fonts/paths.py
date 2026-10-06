"""Filesystem locations used by termux-fonts.

Every path funnels through :func:`termux_dir` so tests can point the whole
application at a temporary directory via the ``TERMUX_HOME`` env var.
"""

from __future__ import annotations

import os
from pathlib import Path

SLOT_FILES: dict[str, str] = {
    "regular": "font.ttf",
    "bold": "font-bold.ttf",
    "italic": "font-italic.ttf",
    "bold-italic": "font-bold-italic.ttf",
}


def termux_dir() -> Path:
    """Return the Termux configuration directory (``~/.termux``)."""
    override = os.environ.get("TERMUX_HOME")
    if override:
        return Path(override).expanduser()
    return Path(os.environ.get("HOME", str(Path.home()))) / ".termux"


def fonts_dir() -> Path:
    """Directory holding the user's font library (``~/.termux/fonts``)."""
    return termux_dir() / "fonts"


def font_slot_path(slot: str) -> Path:
    """Return the active font file for the given slot (e.g. ``regular``)."""
    return termux_dir() / SLOT_FILES[slot]


def backups_dir() -> Path:
    """Where timestamped backups are written."""
    return termux_dir() / "backups"
