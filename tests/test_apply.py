"""Tests for backup-once preview/commit/restore and reload."""

from __future__ import annotations

from pathlib import Path

import pytest

from termux_fonts import apply, paths


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


def _setup_state(tmp_path, monkeypatch, original_family="Orig"):
    _use_termux_home(tmp_path, monkeypatch)
    # Silence real reloads; individual tests override as needed.
    monkeypatch.setattr(apply, "reload_settings", lambda: True)
    target = paths.font_slot_path("regular")
    target.parent.mkdir(parents=True, exist_ok=True)
    _make_font(target, family=original_family)
    original = target.read_bytes()
    st = apply.new_session_state()
    return target, original, st


def test_backup_once_reused(tmp_path, monkeypatch):
    target, _original, st = _setup_state(tmp_path, monkeypatch)
    a = _make_font(tmp_path / "a.ttf", family="A")
    b = _make_font(tmp_path / "b.ttf", family="B")
    apply.preview_font(a, "regular", st)
    apply.preview_font(b, "regular", st)
    backups = paths.backups_dir()
    assert len(list(backups.glob("font-*.ttf"))) == 1


def test_commit_clears_dirty(tmp_path, monkeypatch):
    target, _original, st = _setup_state(tmp_path, monkeypatch)
    a = _make_font(tmp_path / "a.ttf", family="A")
    apply.preview_font(a, "regular", st)
    assert apply.is_preview_dirty(st)
    apply.commit_preview(st)
    assert not apply.is_preview_dirty(st)


def test_restore_after_preview(tmp_path, monkeypatch):
    target, original, st = _setup_state(tmp_path, monkeypatch)
    a = _make_font(tmp_path / "a.ttf", family="A")
    apply.preview_font(a, "regular", st)
    assert target.read_bytes() != original
    assert apply.restore_original(st) is True
    assert target.read_bytes() == original
    assert not apply.is_preview_dirty(st)


def test_invalid_preview_rejected(tmp_path, monkeypatch):
    _target, _original, st = _setup_state(tmp_path, monkeypatch)
    bad = tmp_path / "bad.ttf"
    bad.write_bytes(b"not a font")
    with pytest.raises(ValueError):
        apply.preview_font(bad, "regular", st)


def test_missing_reload_warns_not_raises(tmp_path, monkeypatch):
    # Review Focus: missing termux-reload-settings -> False, file kept
    from termux_fonts.apply import reload_settings as real_reload

    monkeypatch.setattr("shutil.which", lambda _: None)
    assert real_reload() is False


def test_rapid_repreview_single_backup(tmp_path, monkeypatch):
    # Review Focus: 5 rapid previews -> 1 backup, last wins
    target, _original, st = _setup_state(tmp_path, monkeypatch)
    fonts = [_make_font(tmp_path / f"{n}.ttf", family=f"F{n}") for n in ("a", "b", "c", "d", "e")]
    for f in fonts:
        apply.preview_font(f, "regular", st)
    backups = paths.backups_dir()
    assert len(list(backups.glob("font-*.ttf"))) == 1
    assert target.read_bytes() == fonts[-1].read_bytes()


def test_multi_slot_backups_independent(tmp_path, monkeypatch):
    # Cross-slot: previewing bold then regular must keep 2 distinct backups.
    _use_termux_home(tmp_path, monkeypatch)
    monkeypatch.setattr(apply, "reload_settings", lambda: True)
    reg_target = paths.font_slot_path("regular")
    bold_target = paths.font_slot_path("bold")
    reg_target.parent.mkdir(parents=True, exist_ok=True)
    _make_font(reg_target, family="RegOrig")
    _make_font(bold_target, family="BoldOrig")
    reg_orig = reg_target.read_bytes()
    bold_orig = bold_target.read_bytes()
    st = apply.new_session_state()
    new_bold = _make_font(tmp_path / "nb.ttf", family="NB")
    new_reg = _make_font(tmp_path / "nr.ttf", family="NR")
    apply.preview_font(new_bold, "bold", st)
    apply.preview_font(new_reg, "regular", st)
    backups = paths.backups_dir()
    all_bk = sorted(backups.glob("font-*.ttf"))
    assert len(all_bk) == 2
    bold_bk = list(backups.glob("font-bold-*.ttf"))
    assert len(bold_bk) == 1
    assert bold_bk[0].read_bytes() == bold_orig
    reg_bk = [p for p in all_bk if p not in bold_bk]
    assert len(reg_bk) == 1
    assert reg_bk[0].read_bytes() == reg_orig
    assert reg_target.read_bytes() == new_reg.read_bytes()
    assert bold_target.read_bytes() == new_bold.read_bytes()
