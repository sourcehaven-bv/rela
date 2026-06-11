---
id: IMPL-7RWR
type: implementation-checklist
title: 'Implementation: Analyze page warning count out of sync with visible tables (gaps + duplicates hidden)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] ~~Integration tests written (test full flow, not just units)~~ (N/A: pure presentation change; full-flow exercised by existing e2e suite `e2e/tests/analyze.spec.ts` which asserts on `ANALYSIS_CHECKS` and now covers all six categories)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] ~~Error handling in place (errors surfaced, not swallowed)~~ (N/A: no new error paths; this adds two constant entries to a render-side lookup)

## Test Quality

- [x] Using fixture builders or factories for test data
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

Notes on test-quality items: existing `makeIssue`/`makeResult` factories in
`AnalyzeView.test.ts` were used for all three new test cases. Strings
`'Duplicates'` and `'ID Gaps'` are hardcoded as `checkType` values because they
ARE the trigger values for the rendering branches under test (per CLAUDE.md
guidance: "Trigger values: Testing rules that trigger on specific values").

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

Built `rela-server` with the new bundle and ran against the local `tickets/`
rela project (which contains live duplicate titles and ID-sequence gaps).
Visited `/analyze` in a headless browser. Result:

- API returned 3 errors / 45526 warnings; `byCheck` = `{Duplicates: 2,
ID Gaps: 45522, Validations: 5}`.
- AC1 ✓ — Duplicates card rendered with count 2 and two clickable rows
(FEAT-004, FEAT-007 — both pointing at "Duplicate title (shared by FEAT-004,
FEAT-007)").
- AC2 ✓ — ID Gaps card rendered with count 45522 and a row per missing
ID. Sampled rows ("Missing ID: IDEA-006", "Missing ID: RR-1630") show em-dash
placeholders in entity/type cells and no `.clickable` class.
- AC3 ✓ — sum of visible per-card counts (0+0+5+0+2+45522) = 45529 =
errors+warnings on the summary badge.
- AC4 ✓ — existing Properties/Cardinality/Validations/Orphans cards
render unchanged at the top of the page. Existing 5 vitest cases pass.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind

**Automated gates run:**

- `npm run test:run` (frontend) → 45 test files, 787 tests, all pass.
- `npm run typecheck` (frontend, vue-tsc) → clean.
- `npm run lint` (frontend, eslint) → 0 errors (75 pre-existing
warnings, none in changed files).
- `npx tsc --noEmit` (e2e) → clean.
- `go test ./internal/dataentry/` → pass (sanity check; no Go changes).
