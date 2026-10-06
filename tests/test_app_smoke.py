"""Smoke test: picker mounts with empty library; no preview side-effects."""

from __future__ import annotations

import asyncio


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


def test_app_mounts_with_empty_library(tmp_path, monkeypatch):
    from termux_fonts.app import TermuxFontsApp

    monkeypatch.setenv("TERMUX_HOME", str(tmp_path / "termux"))

    async def _check():
        app = TermuxFontsApp()
        async with app.run_test() as pilot:
            await pilot.pause()
            screen = app.screen
            assert screen.query_one("#picker-list") is not None
            preview = screen.query_one("#picker-preview")
            assert preview is not None

    asyncio.run(_check())


def test_enter_after_preview_commits_once(tmp_path, monkeypatch):
    """Enter with the list focused must commit exactly once (no double-fire).

    The screen has both an ("enter", "commit") binding and
    on_list_view_selected -> action_commit; if Textual fired both, the
    second call would see clean state and fall through to install_font
    (extra reload + "Installed" toast overriding "Kept").
    """
    from termux_fonts import apply, paths
    from termux_fonts.app import TermuxFontsApp
    from termux_fonts.screens import FontPickerScreen
    from textual.widgets import ListView

    monkeypatch.setenv("TERMUX_HOME", str(tmp_path / "termux"))
    library = paths.fonts_dir()
    library.mkdir(parents=True)
    _make_font(library / "Alpha.ttf", family="Alpha")
    slot_target = paths.font_slot_path("regular")
    slot_target.parent.mkdir(parents=True, exist_ok=True)
    _make_font(slot_target, family="Orig")

    calls = {"install": 0, "commit": 0, "reload": 0}
    real_install, real_commit, real_reload = (
        apply.install_font,
        apply.commit_preview,
        apply.reload_settings,
    )

    def _spy_install(*args, **kwargs):
        calls["install"] += 1
        return real_install(*args, **kwargs)

    def _spy_commit(*args, **kwargs):
        calls["commit"] += 1
        return real_commit(*args, **kwargs)

    def _spy_reload(*args, **kwargs):
        calls["reload"] += 1
        return real_reload(*args, **kwargs)

    monkeypatch.setattr(apply, "install_font", _spy_install)
    monkeypatch.setattr(apply, "commit_preview", _spy_commit)
    monkeypatch.setattr(apply, "reload_settings", _spy_reload)

    async def _check():
        app = TermuxFontsApp()
        async with app.run_test() as pilot:
            await pilot.pause()
            screen = app.screen
            assert isinstance(screen, FontPickerScreen)
            screen.query_one("#picker-list", ListView).focus()
            await pilot.pause()
            await pilot.press("space")
            await pilot.pause()
            assert apply.is_preview_dirty(screen.state)
            assert calls == {"install": 0, "commit": 0, "reload": 1}
            await pilot.press("enter")
            await pilot.pause()
            assert not apply.is_preview_dirty(screen.state)
            assert calls["commit"] == 1
            assert calls["install"] == 0
            assert calls["reload"] == 1

    asyncio.run(_check())


def test_list_focused_on_mount_so_arrows_move(tmp_path, monkeypatch):
    """Arrows must move the highlight right after launch (no Tab needed).

    Regression: focus used to land in the filter Input, so up/down edited
    the filter and the highlight never moved.
    """
    from termux_fonts import paths
    from termux_fonts.app import TermuxFontsApp
    from textual.widgets import Input, ListView

    monkeypatch.setenv("TERMUX_HOME", str(tmp_path / "termux"))
    library = paths.fonts_dir()
    library.mkdir(parents=True)
    _make_font(library / "Alpha.ttf", family="Alpha")
    _make_font(library / "Beta.ttf", family="Beta")

    async def _check():
        app = TermuxFontsApp()
        async with app.run_test() as pilot:
            await pilot.pause()
            assert isinstance(app.focused, ListView)
            assert not isinstance(app.focused, Input)
            list_view = app.screen.query_one("#picker-list", ListView)
            assert list_view.index == 0
            await pilot.press("down")
            await pilot.pause()
            assert list_view.index == 1

    asyncio.run(_check())
