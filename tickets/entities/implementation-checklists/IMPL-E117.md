---
id: IMPL-E117
type: implementation-checklist
title: 'Implementation: Git-conflict-marker detector matches substring anywhere instead of line-anchored'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] ~~Integration tests written (test full flow, not just units)~~ (N/A: pure-function predicate; unit tests cover the matrix exhaustively)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] Error handling in place (errors surfaced, not swallowed)

## Test Quality

- [x] Using fixture builders or factories for test data
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

Line-anchored conflict-marker detection ships in two places:

- `internal/markdown/parser.go`: `HasConflictMarkers` / `HasConflictMarkersString` now call a shared `hasLineAnchoredMarker` predicate that scans for the marker only at offset 0 or immediately after a newline. The existing `TestParseDocument_ConflictedFile*` tests still pass. New tests `TestParseDocument_ConflictMarkerInCodespan_NotAConflict` (3 cases: inline codespan, indented prose, mid-line YAML scalar) and `TestHasConflictMarkers_LineAnchored` (8 cases including CRLF) pin the line-anchor semantic.
- `internal/store/fsstore/markdown.go`: `parseDocument` now calls a local `hasLineAnchoredConflict` helper with the same semantics. New tests `TestParseDocument_ConflictMarker_LineAnchored` (5 cases) and `TestHasLineAnchoredConflict` (11 cases) cover the matrix.
- `just ci` exits 0 locally after the fix.
- Smoke verification: `rela --project tickets show BUG-WN6D` loads successfully even though `BUG-WN6D.md` body talks about the marker (the original false-positive trigger). `rela --project tickets validate` now surfaces the 42 pre-existing data-debt errors that were silently masked while the detector ate PLAN-ABRRT.md. The TKT-5S8T sweep lands alongside this fix to address all of them.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
