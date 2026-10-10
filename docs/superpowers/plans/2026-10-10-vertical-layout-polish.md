# Vertical Layout + Visual Polish Implementation Plan

**Goal:** Make the picker prettier and move the preview pane on top of the
library list (it previously sat to the side). The user picked a "blend"
mockup: Variant A's calm titled structure plus Variant B's aligned metadata
grid and dotted rules, now arranged as a vertical stack.

**Design (approved in-chat before coding):**

- **Vertical stack** — a full-width `Preview` pane on top, the lower pane
  below: the `Library` list normally, or the `Nerd Fonts` picker while the
  download overlay is open. No more two-column split.
- **Titled borders** — each pane draws its name in the top border
  (`╭─ Library ─…`), so the frame reads as labeled regions.
- **Height split** — the preview takes its natural height, capped at 45%
  of the band and leaving the lower pane a three-row floor. A terminal too
  short for the preview folds it away (`folded`); the list then fills the
  band. Width no longer forces the fold — a narrow-but-tall terminal keeps
  the stack.
- **Preview content** — name + style header, dim dotted rule, the two
  `AaBbCc` samples, Nerd/powerline coverage, a dim rule, a two-column
  metadata grid (`glyphs`, `UPM`, `version`, `slot`, `file`, `backup`), and
  the live shell prompt (mock fallback).
- **Library** — a `⌕` search prompt with a dim right-aligned count, the
  `● slot` badge, and the accent `▶` selection marker.
- **Download** — the picker swaps into the lower pane; its visible window
  shrinks to the rows actually available so the cursor never scrolls off.
- **ASCII** (`--ascii`) — new chrome falls back: `⟨⟩`→`<>`, `⌕`→`?`,
  `┄`→`-`, titled borders `+- Title -+`, and ASCII prompt/coverage samples.

**Files:** `internal/tui/view.go` (styles, `PreviewPane`, `titledBox`,
`rule`, `metaLine`, `spread`, layout + `sizeWidgets`), `internal/tui/model.go`
(pane geometry fields, filter prompts), `internal/tui/download.go`
(`dlWindowRows`), tests in `internal/tui/model_test.go`. Spec §1/§2/§8 and
the README updated to match.

**Tests:** `TestView_VerticalLayout` (stacked, preview above library),
`TestView_VerticalWorksNarrow` (60x24 keeps the stack), `TestView_ShortFoldsPreview`
(a 12-row terminal folds), `TestLayout_PaneLinesFillWidth` (titled border
matches the body width — pins an ANSI/byte-width regression),
`TestDownload_LayoutFitsAndStacks`; the download/narrow/ASCII layout tests
were retargeted from "two columns" to "stacked".

**Completion:** `go build ./...`, `go test ./...`, `go vet ./...` and
`gofmt -l .` are green. Frames were rendered by hand at 60x12 (folded),
60x24, 100x40, the download overlay mid-fetch, and 60x24 ASCII.
