"""Import external font files into the ``~/.termux/fonts`` library."""

from __future__ import annotations

import errno
import shutil
from pathlib import Path

from termux_fonts import paths
from termux_fonts.validate import is_valid_font

_ON_CLASH_MODES: tuple[str, ...] = ("error", "keep-both", "replace")

_STORAGE_HINT = (
    "permission denied: if the file lives on shared storage "
    "(/sdcard/Download/...), run `termux-setup-storage`, grant the storage "
    "permission, and retry"
)


def _storage_error(exc: OSError, where: Path) -> OSError:
    return OSError(f"{_STORAGE_HINT} ({where}: {exc})")


def _raise_for_os_error(exc: OSError, where: Path) -> None:
    if exc.errno == errno.EACCES:
        raise _storage_error(exc, where) from exc
    raise exc


def resolve_clash(dest: Path) -> Path:
    """Return the first free ``<stem>-N<suffix>`` sibling of *dest*."""
    dest = Path(dest)
    index = 1
    while True:
        candidate = dest.with_name(f"{dest.stem}-{index}{dest.suffix}")
        if not candidate.exists():
            return candidate
        index += 1


def import_file(src: Path, on_clash: str = "error") -> Path:
    """Validate *src* and copy it into the font library.

    *on_clash* controls what happens when the library already holds a file
    with the same name: ``"error"`` raises :class:`FileExistsError`,
    ``"keep-both"`` picks ``<name>-N.ttf``, ``"replace"`` overwrites.
    """
    if on_clash not in _ON_CLASH_MODES:
        raise ValueError(
            f"unknown on_clash mode {on_clash!r}; "
            f"choose from {', '.join(_ON_CLASH_MODES)}"
        )
    src = Path(src)
    ok, reason = is_valid_font(src)
    if not ok:
        lowered = reason.lower()
        if "permission denied" in lowered or "errno 13" in lowered:
            raise _storage_error(OSError(reason), src)
        raise ValueError(f"invalid font {src}: {reason}")
    dest_dir = paths.fonts_dir()
    try:
        dest_dir.mkdir(parents=True, exist_ok=True)
    except OSError as exc:
        _raise_for_os_error(exc, dest_dir)
    dest = dest_dir / src.name
    try:
        if dest.exists() and src.resolve() == dest.resolve():
            return dest
    except OSError:
        pass
    if dest.exists():
        if on_clash == "error":
            raise FileExistsError(f"font already in library: {dest.name}")
        if on_clash == "keep-both":
            dest = resolve_clash(dest)
    try:
        shutil.copy2(src, dest)
    except OSError as exc:
        _raise_for_os_error(exc, src)
    return dest
