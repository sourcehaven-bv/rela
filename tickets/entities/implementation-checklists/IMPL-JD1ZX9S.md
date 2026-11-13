---
id: IMPL-JD1ZX9S
type: implementation-checklist
title: 'Implementation: Remote sync status, conflict list and guides'
started: "2026-11-12"
completed: "2026-11-12"
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

- `e2e/tests/remotes.spec.ts` runs the real server on a project with a
remote and a seeded conflict list: the footer link opens the page, the entry on
an unreadable row is neither counted nor shown, each entry shows only its
allowed actions, keeping an uncertain create asks first, and keep local, dismiss
and link each remove their entry. Passes.
- `internal/dataentry/remotes_handler_test.go`: gating, counts after gating,
`_actions` agree with the POSTs, uniform 404, no secrets in errors, read budget
equal at 10 and 50 entries, `/_me` `can_remote_sync`.
- Frontend unit tests for both views and the footer link; the fake-vs-server
contract run passes with the new inert `/_remotes` route.
- Guides generated with `just docs`; the three docs-project gates are clean.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
