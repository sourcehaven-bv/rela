---
id: IMPL-9UKGGQ
type: implementation-checklist
title: 'Implementation: Style markdown-rendered tables in data-entry content views'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code (markdown.test.ts: "renders GFM tables as table/th/td elements" — guards the structural contract the `.md-body` CSS binds to)
- [x] ~~Integration tests written~~ (N/A: CSS-only change; the structural contract is unit-tested and both renderers' GFM output is verified below)
- [x] Happy path implemented (shared `.md-body` table CSS in `frontend/src/styles/markdown-content.css`, applied to all three markdown surfaces)
- [x] Edge cases handled (wide tables scroll via container `overflow-x: auto`; narrow tables keep `width: 100%` fill — no `display:block` shrink-wrap regression)
- [x] ~~Error handling~~ (N/A: pure CSS + class attribute; no error paths)

## Test Quality

- [x] ~~Fixture builders/factories~~ (N/A: single inline markdown string is the input under test)
- [x] No hardcoded values in assertions where an object is in scope (assertions are on rendered HTML fragments, the actual output)
- [x] Only specifying values that matter (asserts `<table>`, one `<th>`, one `<td>` — the structural contract, not full HTML)
- [x] ~~Interpolated values from objects~~ (N/A)
- [x] ~~Property comparisons use original object~~ (N/A)

## Manual Verification

- [x] Feature verified end-to-end (see evidence)
- [x] Each acceptance criterion verified
- [x] Edge cases verified (wide-table scroll, narrow-table fill)

**Verification Evidence:**

- Full frontend suite: `npm run test:run` → 72 files, 1160 tests pass (incl. new table test).
- `npm run typecheck` clean; `npm run lint` 0 errors (only pre-existing warnings, none in touched files); `npm run build` succeeds.
- Compiled CSS in the production bundle confirmed correct:
`.md-body{overflow-x:auto}` · `.md-body
table{border-collapse:collapse;width:100%;margin:16px 0}` · `.md-body
th,.md-body td{...;border:1px solid var(--border-color);padding:8px 12px}` ·
`.md-body th{background:var(--hover-bg);font-weight:600}`.
- Both renderers emit a bare `<table>` that `.md-body table` matches: client `marked` with `gfm:true`
(unit-tested) and server goldmark with `extension.GFM`/`extension.Table`
(`internal/dataentry/document.go:334-335`).
- Live-browser verification was attempted but the Chrome extension was not connected; verified instead via
compiled-CSS inspection + renderer-output confirmation + the structural unit
test. Theming is inherited from existing tokens (`--border-color`,
`--hover-bg`), so light/dark are handled without an override.

## Quality

- [x] Code follows project patterns (global stylesheet + class, mirroring the `styles/back-button.css` / `.scope-nav-btn` precedent)
- [x] Checked for DRY opportunities (removed the duplicated `.document-body :deep(table/th/td)` blocks from DocumentView + DocumentsPanel; the broader heading/code CSS drift is filed as follow-up TKT-W3OPRX rather than over-scoped here)
- [x] No security issues introduced (CSS + static class; v-html/DOMPurify path unchanged)
- [x] No silent failures (N/A — no code paths)
- [x] No debug code left behind (temporary test-table entity edits reverted via `git checkout`)
