"""Core data models for termux-fonts."""

from __future__ import annotations

from dataclasses import dataclass
from pathlib import Path


@dataclass(frozen=True)
class FontEntry:
    """One font file in the user's font library."""

    name: str
    path: Path
    size: int
    mtime: float
    family: str
    style: str
