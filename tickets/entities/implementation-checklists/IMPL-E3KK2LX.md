---
id: IMPL-E3KK2LX
type: implementation-checklist
title: 'Implementation: rela test: stub and record outgoing HTTP calls'
started: "2026-11-14"
completed: "2026-11-14"
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

**Verification Evidence:** Built `rela` and ran `rela test` on a scratch project
whose automation posts to a payments API with a secret. A case with `t.secrets`
and a 503-then-201 stub passed: the script saw 503, `s:count()` was 1 and the
recorded Authorization header held the fake key. A case with only a stub for
another host failed with "unexpected HTTP call POST
https://api.mollie.test/v2/payments; registered stubs:" and the stub listed. The
run showed that a generated stub name repeated the description in that list, so
the list now prints the name only when the test gave one. The acceptance
criteria are pinned by `internal/apptest/httpstub_test.go` (all pass) and the
flowengine, appbuild, lua and cli tests pass.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
