---
id: IMPL-TVI0KY
type: implementation-checklist
title: 'Implementation: Remove the sync feature'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] ~~Integration tests written (test full flow, not just units)~~ (N/A: removal; existing restore e2e tests cover RecreateEntity)
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

- `rela sync` now fails with "unexpected argument sync".
- No Go source imports `internal/sync` or `internal/cli/sync`, or routes
  `/api/sync`.
- `go test ./...` passes on the default, sqlite and memorybackend builds.
  The postgres suite passes; `TestTxPoolExhaustion` failed once under a
  parallel run and passed alone (it counts idle sessions database-wide).
- RecreateEntity tests cover an existing face, a denied face, the face rule,
  a unique collision and a sibling face of another type.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
