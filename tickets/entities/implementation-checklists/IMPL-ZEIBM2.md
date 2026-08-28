---
id: IMPL-ZEIBM2
type: implementation-checklist
title: 'Implementation: Computed properties in schema.yaml: derived, non-editable, stored and indexed, with chained derivation and cycle detection'
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

- `rela validate` accepted a schema with typed integer and string computed fields.
- CLI create materialized `score=12` and `summary="Alpha: scored"`.
- `rela list --filter 'entity.score == 12'` returned the materialized entity.
- Updating `base` recomputed `score` from 12 to 20.
- CLI attempts to set `score` or unset `summary` both failed as read-only.
- Unit/integration coverage includes chaining, cycles, type errors, RRULE portability,
create/update/patch/apply, shape drift, data-entry affordances, and project
validation.
- Race-enabled repository test suite passed; linter passed with zero issues.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
