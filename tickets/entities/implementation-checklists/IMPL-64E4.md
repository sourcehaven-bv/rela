---
id: IMPL-64E4
type: implementation-checklist
title: 'Implementation: affordances: migrate resolver to *acl.Declarative; has_role consults ancestor-conferred roles'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] Integration tests written (test full flow, not just units)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] Error handling in place (errors surfaced, not swallowed)

Code ported from `feat/acl-v1-tkt-svxl` with surgical adjustments:

- Changed: `internal/affordances/{resolver,bindings,hostfuncs,resolver_test,hostfuncs_test}.go`
- Added: `internal/affordances/features_test.go` (UC10/UC11 + ancestor-conferred + AC8 discriminating)
- Deleted: `internal/affordances/effective_roles.go` (subsumed by acl.Request.ForEntity)
- Adjusted for PR-3 compile: `internal/dataentry/affordances_stub.go`
builds Declarative locally with NullGraph (PR 4 swaps for store-graph + new
accessor), `internal/dataentry/affordances_policy_test.go` updated for new
signature.

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

- `go test ./...` — full tree green
- `go test -race ./internal/affordances/... ./internal/dataentry/...` — race-clean
- `just lint` — 0 issues
- `just arch-lint` — clean
- `just coverage-check` — 74.3% (package floors PASS)

AC mapping:

| AC | Test | Result |
|----|------|--------|
| 1 | tree builds with `affordances.New(meta, lookup, *acl.Declarative)` | PASS |
| 2 | `effective_roles.go` not in tree | PASS (`git rm`) |
| 3 | `TestFeature_HasRole_AncestorConferred` (2 subtests) | PASS |
| 4 | `TestResolver_ReusesRequestFromContext` | PASS |
| 5 | `TestFeature_AC8_WriteAffordanceParity` (discriminating) | PASS |
| 6 | full tree green + race-clean | PASS |

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
