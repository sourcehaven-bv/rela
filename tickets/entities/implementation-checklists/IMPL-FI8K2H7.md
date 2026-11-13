---
id: IMPL-FI8K2H7
type: implementation-checklist
title: 'Implementation: Fix _views path in retired-id redirect docs'
started: "2026-11-14"
completed: "2026-11-14"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] ~~Unit tests written for new code~~ (N/A: docs-only change)
- [x] ~~Integration tests written~~ (N/A: docs-only change)
- [x] Happy path implemented
- [x] ~~Edge cases from planning handled~~ (N/A: none identified for a docs fix)
- [x] ~~Error handling in place~~ (N/A: docs-only change)

## Test Quality

- [x] ~~Using fixture builders or factories for test data~~ (N/A: no tests)
- [x] ~~No hardcoded values in assertions when object is in scope~~ (N/A: no tests)
- [x] ~~Only specifying values that matter for the test~~ (N/A: no tests)
- [x] ~~Interpolated values constructed from objects~~ (N/A: no tests)
- [x] ~~Property comparisons use original object~~ (N/A: no tests)

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] ~~Edge cases manually verified~~ (N/A: none identified for a docs fix)

**Verification Evidence:**

`grep -rn "_views/<view>" docs-project docs` returns nothing after the edit and
`just docs`. The guide now writes `/api/v1/_views/<type>/<id>`, which matches
the path parsed by `handleV1Views` in `internal/dataentry/views_handler.go`.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] ~~Checked for DRY opportunities~~ (N/A: one-word docs fix)
- [x] ~~No security issues introduced~~ (N/A: docs-only change)
- [x] ~~No silent failures~~ (N/A: docs-only change)
- [x] No debug code left behind
