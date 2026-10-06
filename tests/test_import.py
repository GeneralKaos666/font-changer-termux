"""Tests for font import and Nerd Font download."""

from __future__ import annotations

import errno
import shutil
from pathlib import Path
from urllib.error import URLError

import pytest

from termux_fonts import downloader, importer, paths


def _use_termux_home(tmp_path, monkeypatch):
    root = tmp_path / "termux"
    monkeypatch.setenv("TERMUX_HOME", str(root))
    return root


def _make_font(path: Path, family: str = "Test") -> Path:
    from fontTools.fontBuilder import FontBuilder
    from fontTools.pens.ttGlyphPen import TTGlyphPen

    fb = FontBuilder(1000)
    fb.setupGlyphOrder([".notdef", "A"])
    fb.setupCharacterMap({65: "A"})

    def _glyph():
        pen = TTGlyphPen(None)
        pen.moveTo((0, 0))
        pen.lineTo((500, 0))
        pen.lineTo((500, 700))
        pen.closePath()
        return pen.glyph()

    fb.setupGlyf({".notdef": TTGlyphPen(None).glyph(), "A": _glyph()})
    fb.setupHorizontalMetrics({".notdef": (500, 0), "A": (500, 0)})
    fb.setupHorizontalHeader()
    fb.setupNameTable({"familyName": family, "styleName": "Regular"})
    fb.setupOS2()
    fb.setupPost()
    fb.save(str(path))
    return path


def test_import_copies_valid(tmp_path, monkeypatch):
    _use_termux_home(tmp_path, monkeypatch)
    dl = tmp_path / "downloads"
    dl.mkdir(parents=True, exist_ok=True)
    src = _make_font(dl / "Hack.ttf", family="Hack")
    dest = importer.import_file(src)
    assert dest.exists()
    assert dest == paths.fonts_dir() / "Hack.ttf"
    assert dest.read_bytes() == src.read_bytes()


def test_import_clash_error_by_default(tmp_path, monkeypatch):
    _use_termux_home(tmp_path, monkeypatch)
    src = _make_font(tmp_path / "Hack.ttf", family="Hack")
    importer.import_file(src)
    with pytest.raises(FileExistsError):
        importer.import_file(src)


def test_import_clash_keep_both(tmp_path, monkeypatch):
    _use_termux_home(tmp_path, monkeypatch)
    src = _make_font(tmp_path / "Hack.ttf", family="Hack")
    first = importer.import_file(src)
    assert first.name == "Hack.ttf"
    second = importer.import_file(src, on_clash="keep-both")
    assert second.name == "Hack-1.ttf"
    assert second.exists()


def test_import_clash_replace(tmp_path, monkeypatch):
    _use_termux_home(tmp_path, monkeypatch)
    src = _make_font(tmp_path / "Hack.ttf", family="Hack")
    importer.import_file(src)
    _make_font(src, family="HackNew")
    dest = importer.import_file(src, on_clash="replace")
    assert dest.name == "Hack.ttf"
    assert dest.read_bytes() == src.read_bytes()


def test_import_invalid_raises(tmp_path, monkeypatch):
    _use_termux_home(tmp_path, monkeypatch)
    bad = tmp_path / "bad.ttf"
    bad.write_bytes(b"not a font")
    with pytest.raises(ValueError):
        importer.import_file(bad)


def test_import_bad_on_clash_raises(tmp_path, monkeypatch):
    _use_termux_home(tmp_path, monkeypatch)
    src = _make_font(tmp_path / "Hack.ttf", family="Hack")
    with pytest.raises(ValueError):
        importer.import_file(src, on_clash="overwrite")


def test_resolve_clash_increments(tmp_path, monkeypatch):
    _use_termux_home(tmp_path, monkeypatch)
    lib = paths.fonts_dir()
    lib.mkdir(parents=True, exist_ok=True)
    dest = lib / "Hack.ttf"
    dest.write_bytes(b"x")
    assert importer.resolve_clash(dest).name == "Hack-1.ttf"
    (lib / "Hack-1.ttf").write_bytes(b"x")
    assert importer.resolve_clash(dest).name == "Hack-2.ttf"


def test_nerd_fonts_has_eleven_entries():
    assert len(downloader.NERD_FONTS) == 11
    for name, url in downloader.NERD_FONTS.items():
        assert url.startswith("https://github.com/ryanoasis/nerd-fonts/raw/v3.2.1/")
        assert url.endswith(".ttf")


def test_fetch_unknown_name_raises(tmp_path, monkeypatch):
    _use_termux_home(tmp_path, monkeypatch)
    with pytest.raises(ValueError):
        downloader.fetch("Not-A-Font")


def test_fetch_downloads_to_library(tmp_path, monkeypatch):
    _use_termux_home(tmp_path, monkeypatch)
    name = next(iter(downloader.NERD_FONTS))

    def fake_retrieve(url, filename, *args, **kwargs):
        assert url == downloader.NERD_FONTS[name]
        _make_font(Path(filename), family="Downloaded")
        return filename, {}

    monkeypatch.setattr(downloader.urllib.request, "urlretrieve", fake_retrieve)
    dest = downloader.fetch(name)
    assert dest.exists()
    assert dest.parent == paths.fonts_dir()
    assert dest.suffix == ".ttf"
    # No leftover partial files after atomic rename.
    assert list(paths.fonts_dir().glob("*.part")) == []


def test_fetch_skips_if_exists(monkeypatch, tmp_path):
    _use_termux_home(tmp_path, monkeypatch)
    name = next(iter(downloader.NERD_FONTS))
    dest_dir = paths.fonts_dir()
    dest_dir.mkdir(parents=True, exist_ok=True)
    from urllib.parse import unquote
    from urllib.request import urlsplit

    url = downloader.NERD_FONTS[name]
    filename = unquote(urlsplit(url).path.rsplit("/", 1)[-1])
    dest = _make_font(dest_dir / filename, family="Cached")

    def fail_if_called(url, filename, *args, **kwargs):  # pragma: no cover
        raise AssertionError("urlretrieve must not be called when same size exists")

    monkeypatch.setattr(downloader.urllib.request, "urlretrieve", fail_if_called)
    monkeypatch.setattr(downloader, "_remote_size", lambda _url: dest.stat().st_size)
    assert downloader.fetch(name) == dest


def test_fetch_force_redownloads(monkeypatch, tmp_path):
    _use_termux_home(tmp_path, monkeypatch)
    name = next(iter(downloader.NERD_FONTS))
    dest_dir = paths.fonts_dir()
    dest_dir.mkdir(parents=True, exist_ok=True)
    from urllib.parse import unquote
    from urllib.request import urlsplit

    url = downloader.NERD_FONTS[name]
    filename = unquote(urlsplit(url).path.rsplit("/", 1)[-1])
    _make_font(dest_dir / filename, family="Stale")
    calls: list[str] = []

    def fake_retrieve(url, filename, *args, **kwargs):
        calls.append(url)
        _make_font(Path(filename), family="Fresh")
        return filename, {}

    monkeypatch.setattr(downloader.urllib.request, "urlretrieve", fake_retrieve)
    dest = downloader.fetch(name, force=True)
    assert calls, "force=True must re-download"
    assert dest.exists()


def test_fetch_partial_cleanup_on_error(monkeypatch, tmp_path):
    _use_termux_home(tmp_path, monkeypatch)
    name = next(iter(downloader.NERD_FONTS))

    def fake_retrieve(url, filename, *args, **kwargs):
        Path(filename).write_bytes(b"partial-bytes")
        raise URLError("network down")

    monkeypatch.setattr(downloader.urllib.request, "urlretrieve", fake_retrieve)
    with pytest.raises(URLError):
        downloader.fetch(name)
    assert list(paths.fonts_dir().glob("*")) == []


def test_import_eacces_hint(tmp_path, monkeypatch):
    # Review Focus: EACCES on /sdcard -> error with termux-setup-storage hint.
    _use_termux_home(tmp_path, monkeypatch)
    src = _make_font(tmp_path / "Hack.ttf", family="Hack")

    def raise_eacces(*args, **kwargs):
        raise PermissionError(errno.EACCES, "Permission denied")

    monkeypatch.setattr(shutil, "copy2", raise_eacces)
    with pytest.raises(OSError, match="termux-setup-storage"):
        importer.import_file(src)

    monkeypatch.undo()
    monkeypatch.setattr(
        "termux_fonts.importer.is_valid_font",
        lambda _p: (False, "cannot read file: [Errno 13] Permission denied"),
    )
    with pytest.raises(OSError, match="termux-setup-storage"):
        importer.import_file(Path("/sdcard/Download/Hack.ttf"))
