---
id: IMPL-FMO7V4O
type: implementation-checklist
title: 'Implementation: List toggle cells: widget toggle with on/off values'
started: "2026-11-24"
completed: "2026-11-24"
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

- [x] ~~Feature manually tested end-to-end~~ (N/A: covered by handler tests over the HTTP list path and vitest on the cell; no browser run)
- [x] Each acceptance criterion verified with test scenario from planning
- [x] ~~Edge cases manually verified~~ (N/A: edge cases are pinned by table tests)

**Verification Evidence:** AC1 validate_toggle_test.go; AC2-4
toggle_rows_test.go (allowed, field grant, transition, hidden, no list_id, other
type, faced row); AC5 querybudget toggle test (10 vs 50 rows, scale-free, fails
without the prime); prime_test.go; AC6 EntityList.toggle.test.ts. Go tests of
changed packages, 236 list vitests, typecheck, arch-lint, plimsoll, comment-lint
and golangci-lint on changed packages pass.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
