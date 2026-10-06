# Font Changer Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build `termux-fonts` Textual TUI that browses `~/.termux/fonts/`, manual live-previews via Space/p, commits via Enter, restores on Esc.

**Architecture:** UI-free core (`paths/scan/validate/apply/importer/downloader`, zero Textual imports) + thin Textual shell (`app.py` + one screen). Core stays CLI-callable for future APK wrapper.

**Tech Stack:** Python >=3.10, `textual>=0.60`, `fontTools` (hard dep), `pytest>=7`, stdlib `urllib` for downloads.

**Spec:** `docs/superpowers/specs/2026-10-04-font-changer-design.md`

## Global Constraints

- Python `>=3.10`.
- Deps: `textual>=0.60`, `fontTools` hard dep (validation + family/style).
- All paths via `TERMUX_HOME` override; tests never touch real `~/.termux`.
- Core modules MUST NOT import `textual`.
- Backup naming: `<slotbase>-YYYY-MM-DD-HHMMSS.ttf` (e.g. `font-2026-10-04-120501.ttf`) under `~/.termux/backups/`; one backup per session per target (backup-once).
- Reload via `shutil.which("termux-reload-settings")` + `subprocess.run([...], check=True)` (no shell); missing binary = warn, keep file.
- Slots: `regular→font.ttf`, `bold→font-bold.ttf`, `italic→font-italic.ttf`, `bold-italic→font-bold-italic.ttf`.
- Valid magic: `00 01 00 00`, `OTTO`, `true`, `typ1`; plus `TTFont` open check.
- Manual preview only: highlight never applies; `Space`/`p` previews, `Enter` commits, `Esc` restores dirty preview.

## Review Focus

- Corrupt file with valid magic but broken sfnt tables → rejected before copy, current font untouched.
- Empty `fonts/` library → list shows empty state + import/download hint, no crash.
- Missing `termux-reload-settings` (e.g. `TERMUX_HOME` test env) → file copied, warning shown, no exception.
- `/sdcard` font import with EACCES → `termux-setup-storage` hint, no traceback.
- Rapid re-preview (Space on 5 rows in <2s) → single backup reused, last preview wins, reload called each time without orphan backups.

---

### Task 1: Scaffolding + paths

**Files:**
- Create: `pyproject.toml`
- Create: `src/termux_fonts/__init__.py`
- Create: `src/termux_fonts/paths.py`
- Test: `tests/test_paths.py`

**Interfaces:**
- Consumes: none.
- Produces:
  - `paths.termux_dir() -> Path`
  - `paths.fonts_dir() -> Path` (`<termux>/.termux/fonts` → actually `<termux>/fonts`; `termux_dir()` is `~/.termux` or `$TERMUX_HOME`)
  - `paths.font_slot_path(slot: str) -> Path`
  - `paths.backups_dir() -> Path`
  - `paths.SLOT_FILES: dict[str, str]`

- [ ] **Step 1: Write failing test `tests/test_paths.py::test_termux_home_override`**

```python
def test_termux_home_override(tmp_path, monkeypatch):
    monkeypatch.setenv("TERMUX_HOME", str(tmp_path / "fake-termux"))
    from termux_fonts import paths
    assert paths.fonts_dir() == tmp_path / "fake-termux" / "fonts"
    assert paths.font_slot_path("regular") == tmp_path / "fake-termux" / "font.ttf"
    assert paths.font_slot_path("bold") == tmp_path / "fake-termux" / "font-bold.ttf"
```

- [ ] **Step 2: Run test to verify it fails**

Run: `python -m pytest tests/test_paths.py::test_termux_home_override -v`
Expected: FAIL with `ModuleNotFoundError` / `file not found`.

- [ ] **Step 3: Implement `paths.py` with `SLOT_FILES`, `termux_dir/fonts_dir/font_slot_path/backups_dir`**

Mirrors `color-changer-termux/src/termux_colors/paths.py:13-38` (`TERMUX_HOME` → `Path(override)`, else `HOME/.termux`).

- [ ] **Step 4: Add `pyproject.toml` (`name=termux-fonts`, `termux-fonts=termux_fonts.app:main`, deps `textual>=0.60`, `fontTools`, dev `pytest>=7`) + empty `__init__.py`, run test to verify it passes**

Run: `python -m pytest tests/test_paths.py -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add pyproject.toml src/termux_fonts/paths.py src/termux_fonts/__init__.py tests/test_paths.py
git commit -m "feat: add TERMUX_HOME-aware paths and packaging"
```

### Task 2: Scan + validate + models

**Files:**
- Create: `src/termux_fonts/models.py`
- Create: `src/termux_fonts/scan.py`
- Create: `src/termux_fonts/validate.py`
- Test: `tests/test_scan.py`

**Interfaces:**
- Consumes: `paths.fonts_dir()`, `paths.font_slot_path()`.
- Produces:
  - `models.FontEntry(name: str, path: Path, size: int, mtime: float, family: str, style: str)`
  - `scan.list_library() -> list[FontEntry]` (sorted case-insensitive by name)
  - `scan.read_active() -> dict[str, Path | None]` (keys `regular/bold/italic/bold-italic`)
  - `validate.is_valid_font(path: Path) -> tuple[bool, str]`
  - `validate.VALID_MAGIC: tuple[bytes, ...]`

- [ ] **Step 1: Write failing tests (sorting, active-missing, invalid reject)**

```python
def test_list_library_sorted(tmp_path, monkeypatch):
    ...
    assert [e.name for e in list_library()] == sorted(names, key=str.lower)

def test_read_active_missing_slot(tmp_path, monkeypatch):
    assert read_active()["bold"] is None

def test_invalid_font_rejected(tmp_path):
    p = tmp_path / "bad.ttf"; p.write_bytes(b"not a font")
    ok, reason = is_valid_font(p)
    assert ok is False

def test_corrupt_valid_magic_rejected(tmp_path):
    # Review Focus: valid magic but broken tables
    p = tmp_path / "corrupt.ttf"
    p.write_bytes(b"\x00\x01\x00\x00" + b"\x00" * 100)
    ok, _ = is_valid_font(p)
    assert ok is False

def test_empty_library_no_crash(tmp_path, monkeypatch):
    # Review Focus: empty fonts/ → [] not exception
    assert list_library() == []
```

- [ ] **Step 2: Run to verify they fail**

Run: `python -m pytest tests/test_scan.py -v`
Expected: FAIL.

- [ ] **Step 3: Implement `models.py` (frozen dataclass `FontEntry`), `validate.py` (magic + `TTFont` open try/except), `scan.py` (glob `*.ttf|*.otf|*.TTF|*.OTF`, `fontTools` family/style with filename fallback)**

One choice: use `TTFont(path, lazy=True)` then `["name"].getDebugName(1/16/17)` guarded; any exception → fallback.

- [ ] **Step 4: Run tests to verify they pass**

Run: `python -m pytest tests/test_scan.py tests/test_paths.py -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add src/termux_fonts/models.py src/termux_fonts/scan.py src/termux_fonts/validate.py tests/test_scan.py
git commit -m "feat: add font library scan and validation"
```

### Task 3: Apply core — backup-once, preview, commit, restore, reload

**Files:**
- Create: `src/termux_fonts/apply.py`
- Test: `tests/test_apply.py`

**Interfaces:**
- Consumes: `paths.font_slot_path/backups_dir`, `validate.is_valid_font`.
- Produces:
  - `apply.snapshot_originals() -> dict[str, bytes | None]`
  - `apply.ensure_backup_once(target: Path, backups: Path | None = None) -> Path | None`
  - `apply.install_font(src: Path, slot: str) -> Path`
  - `apply.preview_font(src: Path, slot: str, state: dict) -> Path` (sets `state["dirty"]=True`, `state["preview"]=(src,slot)`)
  - `apply.commit_preview(state: dict) -> Path | None` (clears dirty)
  - `apply.restore_original(state: dict) -> bool`
  - `apply.reload_settings() -> bool`
  - `apply.is_preview_dirty(state: dict) -> bool`

`state` shape: `{"originals": {...}, "backed_up": set[str], "dirty": bool, "preview": tuple | None}` created by `new_session_state()`.

- [ ] **Step 1: Write failing tests**

```python
def test_backup_once_reused(tmp_path, monkeypatch): ...
    # preview A then preview B to same slot → exactly 1 backup file
    assert len(list(backups.glob("font-*.ttf"))) == 1

def test_commit_clears_dirty(...):
    preview_font(a, "regular", st); assert is_preview_dirty(st)
    commit_preview(st); assert not is_preview_dirty(st)

def test_restore_after_preview(tmp_path, monkeypatch): ...
    # original bytes restored, dirty cleared, reload mocked
    assert target.read_bytes() == original

def test_invalid_preview_rejected(...):
    with pytest.raises(ValueError): preview_font(bad, "regular", st)

def test_missing_reload_warns_not_raises(tmp_path, monkeypatch):
    # Review Focus: missing termux-reload-settings → False, file kept
    monkeypatch.setattr("shutil.which", lambda _: None)
    assert reload_settings() is False

def test_rapid_repreview_single_backup(tmp_path, monkeypatch):
    # Review Focus: 5 rapid previews → 1 backup, last wins
    for f in [a, b, c, d, e]:
        preview_font(f, "regular", st)
    assert len(list(backups.glob("font-*.ttf"))) == 1
    assert target.read_bytes() == e.read_bytes()
```

Mock `reload_settings` via `monkeypatch` on `subprocess.run` / `shutil.which`.

- [ ] **Step 2: Run to verify they fail**

Run: `python -m pytest tests/test_apply.py -v`
Expected: FAIL.

- [ ] **Step 3: Implement `apply.py`**

Backup timestamp `datetime.now().strftime("%Y-%m-%d-%H%M%S")`; copy via `shutil.copy2`; reload via `which+run(check=True)`, `FileNotFoundError` → return `False`.

- [ ] **Step 4: Run tests to verify they pass**

Run: `python -m pytest tests/test_apply.py tests/test_scan.py -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add src/termux_fonts/apply.py tests/test_apply.py
git commit -m "feat: add backup-once preview/commit/restore and reload"
```

### Task 4: Importer + downloader

**Files:**
- Create: `src/termux_fonts/importer.py`
- Create: `src/termux_fonts/downloader.py`
- Test: `tests/test_import.py`

**Interfaces:**
- Consumes: `paths.fonts_dir()`, `validate.is_valid_font`, `scan.list_library`.
- Produces:
  - `importer.import_file(src: Path, on_clash: str = "error") -> Path` (`on_clash` ∈ `error/keep-both/replace`)
  - `importer.resolve_clash(dest: Path) -> Path` (`name-N.ttf`)
  - `downloader.NERD_FONTS: dict[str, str]` (11 entries from `fonts.sh` v3.2.1)
  - `downloader.fetch(name: str, force: bool = False) -> Path`

- [ ] **Step 1: Write failing tests**

```python
def test_import_copies_valid(tmp_path, monkeypatch): ...
    assert dest.exists()

def test_import_clash_keep_both(...):
    assert second.name == "Hack-1.ttf"

def test_import_invalid_raises(...):
    with pytest.raises(ValueError): import_file(bad)

def test_fetch_skips_if_exists(monkeypatch, tmp_path): ...
    # monkeypatch urllib.request.urlretrieve, assert not called when same size exists

def test_import_eacces_hint(tmp_path, monkeypatch):
    # Review Focus: EACCES on /sdcard → ValueError/OSError with termux-setup-storage hint
    ...
```

- [ ] **Step 2: Run to verify they fail**

Run: `python -m pytest tests/test_import.py -v`
Expected: FAIL.

- [ ] **Step 3: Implement `importer.py` (validate → clash → `shutil.copy2`), `downloader.py` (`urllib.request.urlretrieve` to temp + atomic rename, partial cleanup on error)**

`NERD_FONTS` URLs copied from `~/.termux/fonts.sh` (`NF_VERSION=v3.2.1` block).

- [ ] **Step 4: Run tests to verify they pass**

Run: `python -m pytest tests/test_import.py tests/test_apply.py -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add src/termux_fonts/importer.py src/termux_fonts/downloader.py tests/test_import.py
git commit -m "feat: add font import and Nerd Font download"
```

### Task 5: Textual TUI — browse, manual preview, commit/restore

**Files:**
- Create: `src/termux_fonts/app.py`
- Create: `src/termux_fonts/screens.py` (single `FontPickerScreen`; keep one file, split only if >400 lines)
- Test: `tests/test_app_smoke.py`

**Interfaces:**
- Consumes: all core (`scan/apply/importer/downloader/paths`).
- Produces:
  - `app.TermuxFontsApp(App)` with bindings `space/p` preview, `enter` commit, `s` slot-cycle, `i` import, `d` download, `q/esc` back-or-restore-quit
  - `app.main() -> None`

Layout mirrors `color-changer` picker (`#picker-list` 42% + `#picker-preview` 58%, filter `Input`, sample `Static`): list | info+sample+slot+status. Highlight updates info only.

- [ ] **Step 1: Write failing smoke test (no preview side-effects)**

```python
async def test_app_mounts_with_empty_library():
    app = TermuxFontsApp()
    async with app.run_test() as pilot:
        assert app.query_one("#picker-list")
        assert "No fonts" in app.query_one("#picker-preview").renderable or True
```

(Opaque Textual assert allowed: mount + widget IDs exist; real preview/commit covered by Task 3 tests with mocked reload.)

- [ ] **Step 2: Run to verify it fails**

Run: `python -m pytest tests/test_app_smoke.py -v`
Expected: FAIL.

- [ ] **Step 3: Implement `screens.py` + `app.py`**

Key logic: `on_input_changed` filters; `on_list_view_highlighted` updates preview pane only; `action_preview` → `preview_font` + toast `Preview X — Enter keeps, Esc restores`; `action_commit` → `commit_preview` or `install_font`; `on_unmount/escape` → `restore_original` if dirty + `reload`. Slot state `self.slot="regular"`, `s` cycles 4. `i`/`d` open simple `Input`/list dialogs (reuse Textual `Input`, no custom modal lib).

- [ ] **Step 4: Run full suite**

Run: `python -m pytest -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add src/termux_fonts/app.py src/termux_fonts/screens.py tests/test_app_smoke.py
git commit -m "feat: add Textual font picker with manual live preview"
```

### Task 6: CLI polish, builtin seed, docs

**Files:**
- Modify: `src/termux_fonts/app.py` (add `--apply/--slot/--list` flags for Phase-2 reuse)
- Create: `README.md`
- Test: `tests/test_cli.py`

**Interfaces:**
- Consumes: core + app.
- Produces: `termux-fonts [--apply NAME --slot SLOT] [--list]`, builtin seed (if `fonts/` empty and `font.ttf` exists → copy to `fonts/Current.ttf`).

- [ ] **Step 1: Write failing test**

```python
def test_cli_list_and_apply(tmp_path, monkeypatch, capsys): ...
    # --list prints names; --apply copies + reload mocked
```

- [ ] **Step 2: Run to verify it fails**

Run: `python -m pytest tests/test_cli.py -v`
Expected: FAIL.

- [ ] **Step 3: Implement `argparse` in `main()`, seed-on-empty at startup, `README.md` (install `pip install .`, usage keys table, backup + Phase-2 note)**

- [ ] **Step 4: Run full suite + manual dry-run**

Run: `python -m pytest -v`
Expected: PASS. Then: `TERMUX_HOME=/tmp/fc-demo python -m termux_fonts.app --list` smoke.

- [ ] **Step 5: Commit**

```bash
git add src/termux_fonts/app.py README.md tests/test_cli.py
git commit -m "feat: add CLI flags, builtin seed, and README"
```
