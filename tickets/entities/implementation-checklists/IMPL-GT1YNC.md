---
id: IMPL-GT1YNC
type: implementation-checklist
title: 'Implementation: rela-desktop runs on the SQLite backend and opens self-contained projects'
started: "2026-10-04"
completed: "2026-10-04"
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

- The desktop app was built and run during development: documents created
from a template, opened from the welcome screen and recent list, settings and
secrets edited, search used.
- `just test`, `just lint` and `just coverage-check` pass (2026-10-04).
- Store conformance suites, including search, pass with the FTS5 backend.
- Race tests pass on the default and sqlite builds for the changed packages.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
