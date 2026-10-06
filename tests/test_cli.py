"""Tests for the termux-fonts CLI flags and builtin seed."""

from __future__ import annotations

from termux_fonts import apply, app, paths


def _use_termux_home(tmp_path, monkeypatch):
    root = tmp_path / "termux"
    monkeypatch.setenv("TERMUX_HOME", str(root))
    return root


def _make_font(path, family="Test"):
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


def test_cli_list_prints_names(tmp_path, monkeypatch, capsys):
    _use_termux_home(tmp_path, monkeypatch)
    lib = paths.fonts_dir()
    lib.mkdir(parents=True)
    _make_font(lib / "Alpha.ttf", family="Alpha")
    _make_font(lib / "Beta.ttf", family="Beta")
    monkeypatch.setattr("sys.argv", ["termux-fonts", "--list"])
    app.main()
    out = capsys.readouterr().out
    assert "Alpha.ttf" in out
    assert "Beta.ttf" in out


def test_cli_apply_copies_and_reloads(tmp_path, monkeypatch):
    _use_termux_home(tmp_path, monkeypatch)
    monkeypatch.setattr(apply, "reload_settings", lambda: True)
    lib = paths.fonts_dir()
    lib.mkdir(parents=True)
    _make_font(lib / "Chosen.ttf", family="Chosen")
    monkeypatch.setattr("sys.argv", ["termux-fonts", "--apply", "Chosen.ttf"])
    app.main()
    target = paths.font_slot_path("regular")
    assert target.is_file()
    assert target.read_bytes() == (lib / "Chosen.ttf").read_bytes()


def test_builtin_seed_copies_font_ttf(tmp_path, monkeypatch, capsys):
    root = _use_termux_home(tmp_path, monkeypatch)
    slot_file = root / "font.ttf"
    slot_file.parent.mkdir(parents=True, exist_ok=True)
    _make_font(slot_file, family="Builtin")
    # fonts/ empty (does not exist); seed should copy font.ttf -> fonts/Current.ttf
    monkeypatch.setattr("sys.argv", ["termux-fonts", "--list"])
    app.main()
    seeded = paths.fonts_dir() / "Current.ttf"
    assert seeded.is_file()
    assert seeded.read_bytes() == slot_file.read_bytes()
    assert "Current.ttf" in capsys.readouterr().out
