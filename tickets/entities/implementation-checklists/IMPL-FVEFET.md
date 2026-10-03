---
id: IMPL-FVEFET
type: implementation-checklist
title: 'Implementation: Rename affordance and doc ACL assertions decide rename and delete per face'
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

- [x] ~~Feature manually tested end-to-end~~ (N/A: verified by HTTP-level and e2e tests instead of a manual pass)
- [x] ~~Each acceptance criterion verified with test scenario from planning~~ (N/A: bugs have no planning checklist; the Expected section is verified by the regression tests)
- [x] ~~Edge cases manually verified~~ (N/A: edge cases are covered by the table-driven tests)

**Verification Evidence:** `TestFaceGrant_RenameAffordanceIsFamilyWide`
(internal/dataentry) failed before the fix: rename was offered with update on
the published face only. It passes now. `internal/docs/claimfaces_test.go`
covers rename, family delete and per-face create/update claims. `go test ./...`
passes on the default, postgres and sqlite builds.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
