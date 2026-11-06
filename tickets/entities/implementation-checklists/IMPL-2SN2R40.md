---
id: IMPL-2SN2R40
type: implementation-checklist
title: 'Implementation: Real-time co-editing of entity bodies (single node)'
started: "2026-11-05"
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

- `e2e/tests/collab.spec.ts` drives two browsers (alice and bob) against a
  real server: each sees the other's cursor, typing in different paragraphs
  converges, concurrent typing at the start and end of one line converges,
  the stored markdown equals the shared text, and after the save fallback
  only one page has sent saves. Three runs in a row pass.
- Writing the E2E test found two bugs, both fixed with a unit test that fails
  without the fix: a peer's change and an undo never reached the save path,
  and Undo stayed disabled after the first edit.
- `internal/dataentry/collab_test.go` covers the relay over real WebSockets:
  join needs read access (hidden and missing give the same 404), a read-only
  peer cannot write, the stale-build close, lifetime and credential expiry,
  and a Configure save closing connections with the rejoin code. Passes under
  the race detector.
- `collabRoundTrip.test.ts` passes on the whole corpus
  (`RELA_FULL_CORPUS=1`), including entity refs inside marks and table
  alignment; per-principal reference titles never enter the room.
- `collabRoom.test.ts`, `useBodyCollab` tests, `useAutoSave.conflicts.test.ts`
  and `MilkdownEditorCollab.test.ts` cover saver election, the non-saver
  fallback, expiry, join reconcile and the 412 resend rule.
- Full checks: frontend typecheck, lint (0 errors) and 5961 unit tests;
  arch-lint; golangci-lint; comment lint; coverage 85.7%.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
