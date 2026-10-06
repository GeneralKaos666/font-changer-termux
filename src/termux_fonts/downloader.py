"""Download Nerd Fonts into the ``~/.termux/fonts`` library.

URLs mirror ``~/.termux/fonts.sh`` (``NF_VERSION=v3.2.1``): the ``URL_NF``
base plus each ``NF_*`` path. Downloads go to a ``.part`` temp file in the
library directory, then atomically rename into place; partial files are
removed on any error.
"""

from __future__ import annotations

import os
import tempfile
import urllib.parse
import urllib.request
from pathlib import Path

from termux_fonts import paths
from termux_fonts.validate import is_valid_font

_NF_BASE = "https://github.com/ryanoasis/nerd-fonts/raw/v3.2.1/patched-fonts"

NERD_FONTS: dict[str, str] = {
    "JetBrainsMono-Light": f"{_NF_BASE}/JetBrainsMono/Ligatures/Light/JetBrainsMonoNerdFont-Light.ttf",
    "JetBrainsMono-Regular": f"{_NF_BASE}/JetBrainsMono/Ligatures/Regular/JetBrainsMonoNerdFont-Regular.ttf",
    "Hack-Regular": f"{_NF_BASE}/Hack/Regular/HackNerdFont-Regular.ttf",
    "FiraCode-Regular": f"{_NF_BASE}/FiraCode/Regular/FiraCodeNerdFont-Regular.ttf",
    "SourceCodePro-Regular": f"{_NF_BASE}/SourceCodePro/SauceCodeProNerdFont-Regular.ttf",
    "IosevkaTerm-Regular": f"{_NF_BASE}/IosevkaTerm/IosevkaTermNerdFont-Regular.ttf",
    "Mononoki-Regular": f"{_NF_BASE}/Mononoki/Regular/MononokiNerdFont-Regular.ttf",
    "Terminus-Regular": f"{_NF_BASE}/Terminus/TerminessNerdFont-Regular.ttf",
    "CascadiaCode-Regular": f"{_NF_BASE}/CascadiaCode/Regular/CaskaydiaCoveNerdFont-Regular.ttf",
    "IBMPlexMono-Regular": f"{_NF_BASE}/IBMPlexMono/Mono/BlexMonoNerdFontMono-Regular.ttf",
    "AnonymousPro-Regular": f"{_NF_BASE}/AnonymousPro/Regular/AnonymiceProNerdFont-Regular.ttf",
}


def _remote_size(url: str) -> int | None:
    """Return the remote ``Content-Length``, or ``None`` when unknown."""
    try:
        request = urllib.request.Request(url, method="HEAD")
        with urllib.request.urlopen(request) as response:
            length = response.headers.get("Content-Length")
        return int(length) if length is not None else None
    except Exception:  # noqa: BLE001 - size check is best-effort; download decides
        return None


def _dest_for(url: str, name: str) -> Path:
    filename = urllib.parse.unquote(
        urllib.request.urlsplit(url).path.rsplit("/", 1)[-1]
    )
    if not filename:
        filename = f"{name}.ttf"
    return paths.fonts_dir() / filename


def fetch(name: str, force: bool = False) -> Path:
    """Download Nerd Font *name* into the library and return its path.

    Skips the download when the file already exists with the same size as
    the remote (unless *force* is true). Raises :class:`ValueError` for
    unknown names or invalid downloads.
    """
    try:
        url = NERD_FONTS[name]
    except KeyError:
        choices = ", ".join(sorted(NERD_FONTS))
        raise ValueError(f"unknown font {name!r}; choose from: {choices}") from None
    dest_dir = paths.fonts_dir()
    dest_dir.mkdir(parents=True, exist_ok=True)
    dest = _dest_for(url, name)
    if dest.exists() and not force:
        remote = _remote_size(url)
        if remote is not None and remote == dest.stat().st_size:
            return dest
    fd, tmp_name = tempfile.mkstemp(
        prefix=f"{dest.stem}.", suffix=".part", dir=str(dest_dir)
    )
    os.close(fd)
    tmp = Path(tmp_name)
    try:
        urllib.request.urlretrieve(url, str(tmp))
        ok, reason = is_valid_font(tmp)
        if not ok:
            raise ValueError(f"downloaded file for {name!r} is not a valid font: {reason}")
        os.replace(tmp, dest)
    except Exception:
        tmp.unlink(missing_ok=True)
        raise
    return dest
