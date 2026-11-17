---
id: IMPL-UMVEO1O
type: implementation-checklist
title: 'Implementation: Read-time today and now in every query surface'
started: "2026-11-16"
completed: "2026-11-17"
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
<!-- Document what you tested and the results -->
On 2026-11-16 (UTC), with a rela built from the branch, against `tickets/`:
`list implementation-checklist --where 'started=$today'`, `--where
'started>$today-1d'` and `--filter "entity.started > date_add(today(), -1,
'day') and entity.started < now()"` each returned the two checklists started
that day. The data-entry search (`prop:due=$today`, `$today+7d`, `$now`), list
`condition:` on fs, sqlite and postgres, scheduler, validation and Lua paths are
covered by table tests.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
