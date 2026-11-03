---
id: IMPL-8B58Z6Q
type: implementation-checklist
title: 'Implementation: Verified fake API and property tests for SPA editing'
started: "2026-10-29"
completed: "2026-10-29"
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

- `just contract-fake`: 6 tests pass (about 46 s) against `rela-server -tags memorybackend`.
- Contract mutation checks: dropping the 412, a token that ignores the value, no `_versions` on PATCH, and a silent live round each fail the run.
- `EntityDetail.switching.prop.test.ts`, switching property: fails on the unfixed component with the shrunk counterexample `[deliver, deliver, edit, select 1]` (page shows NOTE-001 while the route is NOTE-002); passes after the fix at 50 runs and twice at 300 runs.
- Title property: a mutant where autosave addresses `props.entityId` fails with `[select 1, type, commit]`; 13 of 50 runs commit a title (measured).
- Full unit suite: 353 files, 5317 tests pass. `npm run typecheck` clean; eslint on changed files has no errors.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
