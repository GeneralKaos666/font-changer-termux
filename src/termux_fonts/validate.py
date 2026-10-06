"""Font file validation.

A file is accepted as a font when its 4-byte sfnt signature matches a known
magic value *and* ``fontTools`` can open it and find its ``name`` table.
Checking the magic alone is not enough: a corrupt file can carry a valid
signature while having broken tables (opening it with ``TTFont`` alone does
not raise either, so a table lookup is forced).
"""

from __future__ import annotations

from pathlib import Path

from fontTools.ttLib import TTFont

VALID_MAGIC: tuple[bytes, ...] = (
    b"\x00\x01\x00\x00",  # TrueType / OpenType with TrueType outlines
    b"OTTO",  # OpenType with CFF outlines
    b"true",  # legacy Mac TrueType
    b"typ1",  # PostScript Type 1 (sfnt wrapper)
)


def is_valid_font(path: Path) -> tuple[bool, str]:
    """Return ``(True, "")`` if *path* is a readable font, else ``(False, reason)``."""
    path = Path(path)
    if not path.exists():
        return False, "file does not exist"
    if not path.is_file():
        return False, "not a regular file"
    try:
        header = path.read_bytes()[:4]
    except OSError as exc:
        return False, f"cannot read file: {exc}"
    if len(header) < 4:
        return False, "file is too small to be a font"
    if header not in VALID_MAGIC:
        return False, f"unknown font signature: {header!r}"
    try:
        with TTFont(str(path), lazy=True) as font:
            font["name"]  # force table lookup; raises if tables are broken
    except KeyError:
        return False, "font has no name table (corrupt or unsupported)"
    except Exception as exc:  # noqa: BLE001 - any parse failure means invalid
        return False, f"cannot parse font: {exc}"
    return True, ""
