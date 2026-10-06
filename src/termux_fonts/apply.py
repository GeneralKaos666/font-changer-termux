"""Preview/commit/restore core with backup-once and settings reload."""

from __future__ import annotations

import shutil
import subprocess
from datetime import datetime
from pathlib import Path

from termux_fonts import paths
from termux_fonts.validate import is_valid_font


def snapshot_originals() -> dict[str, bytes | None]:
    """Read current slot files; ``None`` for empty slots."""
    originals: dict[str, bytes | None] = {}
    for slot in paths.SLOT_FILES:
        target = paths.font_slot_path(slot)
        originals[slot] = target.read_bytes() if target.is_file() else None
    return originals


def new_session_state() -> dict:
    """Create fresh preview-session state."""
    return {
        "originals": snapshot_originals(),
        "backed_up": set(),
        "dirty": False,
        "preview": None,
    }


def ensure_backup_once(target: Path, backups: Path | None = None) -> Path | None:
    """Back up *target* once; reuse the existing backup on repeat calls.

    Returns the backup path, or ``None`` when *target* does not exist.
    """
    target = Path(target)
    if not target.is_file():
        return None
    dest_dir = Path(backups) if backups is not None else paths.backups_dir()
    dest_dir.mkdir(parents=True, exist_ok=True)
    existing = sorted(dest_dir.glob(f"{target.stem}-*{target.suffix}"))
    if existing:
        return existing[0]
    timestamp = datetime.now().strftime("%Y-%m-%d-%H%M%S")
    dest = dest_dir / f"{target.stem}-{timestamp}{target.suffix}"
    if dest.exists():
        return dest
    shutil.copy2(target, dest)
    return dest


def _checked_target(src: Path, slot: str) -> Path:
    if slot not in paths.SLOT_FILES:
        raise ValueError(f"unknown slot: {slot!r}")
    ok, reason = is_valid_font(Path(src))
    if not ok:
        raise ValueError(f"invalid font: {reason}")
    return paths.font_slot_path(slot)


def install_font(src: Path, slot: str) -> Path:
    """Validate *src*, back up the slot once, install, and reload."""
    target = _checked_target(src, slot)
    target.parent.mkdir(parents=True, exist_ok=True)
    ensure_backup_once(target)
    shutil.copy2(src, target)
    reload_settings()
    return target


def preview_font(src: Path, slot: str, state: dict) -> Path:
    """Install *src* as a dirty preview; exactly one backup per slot."""
    target = _checked_target(src, slot)
    target.parent.mkdir(parents=True, exist_ok=True)
    ensure_backup_once(target)
    state.setdefault("backed_up", set()).add(slot)
    shutil.copy2(src, target)
    state["dirty"] = True
    state["preview"] = (Path(src), slot)
    reload_settings()
    return target


def commit_preview(state: dict) -> Path | None:
    """Clear the dirty flag, keeping the previewed file."""
    if not state.get("dirty") or state.get("preview") is None:
        return None
    _src, slot = state["preview"]
    target = paths.font_slot_path(slot)
    state["dirty"] = False
    state["preview"] = None
    return target


def restore_original(state: dict) -> bool:
    """Restore original bytes for the previewed slot (or all if unknown)."""
    preview = state.get("preview")
    if preview is None and not state.get("dirty"):
        return False
    slots = [preview[1]] if preview is not None else list(state.get("originals", {}))
    for slot in slots:
        if slot not in paths.SLOT_FILES:
            continue
        target = paths.font_slot_path(slot)
        original = state.get("originals", {}).get(slot)
        if original is None:
            if target.is_file() or target.is_symlink():
                target.unlink()
        else:
            target.parent.mkdir(parents=True, exist_ok=True)
            target.write_bytes(original)
        backed = state.get("backed_up")
        if isinstance(backed, set):
            backed.discard(slot)
    state["dirty"] = False
    state["preview"] = None
    reload_settings()
    return True


def reload_settings() -> bool:
    """Ask Termux to reload settings; ``False`` when the binary is missing."""
    binary = shutil.which("termux-reload-settings")
    if binary is None:
        return False
    try:
        subprocess.run([binary], check=True)
    except FileNotFoundError:
        return False
    except subprocess.CalledProcessError:
        return False
    except OSError:
        return False
    return True


def is_preview_dirty(state: dict) -> bool:
    """Return ``True`` when a preview is awaiting commit/restore."""
    return bool(state.get("dirty", False))
