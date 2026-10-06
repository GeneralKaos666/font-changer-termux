"""Tests for font library scanning, active-slot reading, and validation."""

from termux_fonts import paths
from termux_fonts.scan import list_library, read_active
from termux_fonts.validate import is_valid_font


def _use_termux_home(tmp_path, monkeypatch):
    root = tmp_path / "termux"
    monkeypatch.setenv("TERMUX_HOME", str(root))
    return root


def test_list_library_sorted(tmp_path, monkeypatch):
    _use_termux_home(tmp_path, monkeypatch)
    fonts = paths.fonts_dir()
    fonts.mkdir(parents=True)
    names = ["Zebra.ttf", "apple.ttf", "Mango.OTF"]
    for n in names:
        (fonts / n).write_bytes(b"\x00" * 16)
    assert [e.name for e in list_library()] == sorted(names, key=str.lower)


def test_read_active_missing_slot(tmp_path, monkeypatch):
    _use_termux_home(tmp_path, monkeypatch)
    assert read_active()["bold"] is None


def test_invalid_font_rejected(tmp_path):
    p = tmp_path / "bad.ttf"
    p.write_bytes(b"not a font")
    ok, reason = is_valid_font(p)
    assert ok is False


def test_corrupt_valid_magic_rejected(tmp_path):
    # Review Focus: valid magic but broken tables
    p = tmp_path / "corrupt.ttf"
    p.write_bytes(b"\x00\x01\x00\x00" + b"\x00" * 100)
    ok, _ = is_valid_font(p)
    assert ok is False


def test_empty_library_no_crash(tmp_path, monkeypatch):
    # Review Focus: empty fonts/ -> [] not exception
    _use_termux_home(tmp_path, monkeypatch)
    paths.fonts_dir().mkdir(parents=True)
    assert list_library() == []
