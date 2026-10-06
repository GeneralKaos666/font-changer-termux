"""Smoke test: picker mounts with empty library; no preview side-effects."""

from __future__ import annotations

import asyncio


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
