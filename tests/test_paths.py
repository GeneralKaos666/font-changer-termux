def test_termux_home_override(tmp_path, monkeypatch):
    monkeypatch.setenv("TERMUX_HOME", str(tmp_path / "fake-termux"))
    from termux_fonts import paths
    assert paths.fonts_dir() == tmp_path / "fake-termux" / "fonts"
    assert paths.font_slot_path("regular") == tmp_path / "fake-termux" / "font.ttf"
    assert paths.font_slot_path("bold") == tmp_path / "fake-termux" / "font-bold.ttf"
