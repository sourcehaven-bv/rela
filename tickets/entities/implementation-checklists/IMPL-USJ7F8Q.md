---
id: IMPL-USJ7F8Q
type: implementation-checklist
title: 'Implementation: Bound the structured search path the way free-text already is'
started: "2026-11-23"
completed: "2026-11-23"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] Integration tests written (test full flow, not just units)
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

- Ran `rela-server` on the tickets project (9603 review-responses). `/api/v1/_search?q=type:review-response` returns 1000 rows with total 9603 and has_more; page 10 returns 603 rows; page 11 is empty with total 9603.
- `limit=5&page=3` returns rows 11 to 15; `page=0` returns 400 `invalid_page`.
- The union of every page has 9603 unique ids on the plan path and 10767 on a declined multi-type sort.
- SearchView shows the truncation notice ("De eerste 1000 van 9603 resultaten").
- Go tests: default, memorybackend, sqlite and postgres tags. Frontend: `npm run test:run` and typecheck clean.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
