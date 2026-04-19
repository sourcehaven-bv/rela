---
id: IMPL-G7D6J
type: implementation-checklist
title: 'Implementation: Metamodel parsing of encrypted: declarations + groups config (slice 2)'
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

- `go test ./internal/metamodel/ -race` passes on all 40+ new test cases.
- New-code coverage: 100% on `groups.go`, `groups_errors.go`, `entity_def.go` additions, `validation.go` additions, `loader.go` additions.
- `LoadWithGroups` end-to-end fixtures exercise the full flow: metamodel + groups.yaml present → loads; encryption declared but groups.yaml missing → `ErrGroupsNotFound`; unknown group reference → `ErrUnknownGroup` with correct path.
- `golangci-lint v1.64.8` clean.
- `go-arch-lint` clean — no `internal/encryption` import from metamodel (layering preserved).

## Quality

- [x] Code follows project patterns (check similar code)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
