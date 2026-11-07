---
id: IMPL-I54GLK4
type: implementation-checklist
title: 'Implementation: tenant: assemble every tenant over one SharedBase so config is parsed once'
started: "2026-11-07"
completed: "2026-11-07"
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

- [x] ~~Using fixture builders or factories for test data~~ (N/A: tests reuse the existing newSharedBase and writeIsolationProject helpers)
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] ~~Interpolated values constructed from objects, not hardcoded~~ (N/A: no interpolated values)
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] ~~Feature manually tested end-to-end~~ (N/A: no host wires the registry yet; covered by the postgres isolation suite against a real database)
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:** AC1: `TestTenantIsolation_OneTenantCannotReadAnother`
asserts shared `Meta()` and `ACLPolicy()` pointers. Made non-vacuous by
temporarily re-preparing per tenant: the test failed on both assertions. The
base carries a decoy DSN, so a tenant opened on the wrong DSN fails the leak
assertions. AC2: `go test -race -tags postgres ./internal/tenant/
./internal/appbuild/...` and the default build pass. AC3: Registry doc updated.
Edge cases: empty DSN, reassembly base, non-postgres builds refuse (tests in
sharedbase_test.go and openassemble_local_test.go).

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
