---
id: IMPL-J02X92
type: implementation-checklist
title: 'Implementation: Unify the two entity-ID validators into one enforced rule'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] Integration tests written — storetest conformance runs the rule through every backend, not just the validator in isolation
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] Error handling in place (errors surfaced, not swallowed)

## Test Quality

- [x] ~~Using fixture builders or factories for test data~~ (N/A: table-driven string inputs; no object graph to build)
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

- `go test ./...` — full suite passes.
- `TestValidateID` (entity): 6 accept cases, 24 reject cases, each asserting
  the error message names the reason rather than merely erroring.
- `TestValidateID` (storeutil): rejection of multi-byte UTF-8, leading dash,
  and shell metacharacters, via the delegating path.
- Fuzz (the load-bearing check — `storetest/fuzz.go` uses ValidateID as a
  **bidirectional** oracle, so tightening the rule required the backends to
  actually reject the newly-invalid IDs):
  - `FuzzValidateID` 30s — 6.1M execs, pass
  - `FuzzRelationKeyCollision` 25s — 983k execs, pass
  - `FuzzRenameKeyCollapse` 25s — 933k execs, pass
- Pre-fix reproduction (memstore): `-rf`, `a b`, `a;b`, `a$(id)`, `аdmin`,
  `a*b` all created and retrieved successfully. Post-fix: all rejected.
- Back-compat measured, not assumed: scanned all 2030 entities across
  `tickets/` and `docs-project/` — zero would be rejected.
- `golangci-lint` on changed packages — 0 issues.
- `just arch-lint` — OK. `just coverage-check` — pass (76.8%).

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
