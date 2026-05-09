---
id: IMPL-J6YL2
type: implementation-checklist
title: 'Implementation: Extend PATCH /entities/{id} relations to carry per-edge meta + content'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code (relations_v1_wire_test.go: 17 tests)
- [x] Integration tests written (relations_modern_test.go: 22 ACs)
- [x] Happy path implemented (Layers 0-7 per PLAN-MXQKI)
- [x] Edge cases from planning handled (null/empty/whitespace/scalar/mixed-shape)
- [x] Error handling in place (wireError, structuralError, relationError typed)

## Test Quality

- [x] Using fixture builders (newRelationsTestApp seeds canonical entities)
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] Each acceptance criterion verified — see relations_modern_test.go AC1–AC20
- [x] Edge cases verified via relations_v1_wire_test.go

**Verification Evidence:**

- `go test ./...` — all packages pass
- `go test -race ./...` — no data races
- `golangci-lint run ./...` — clean (0 issues)
- `just coverage-check` — both thresholds PASS
- 22 integration tests in `internal/dataentry/relations_modern_test.go` cover AC1–AC20a + AC15/16
- 17 unit tests in `internal/dataentry/relations_v1_wire_test.go` cover wire-format edge cases

## Quality

- [x] Code follows project patterns (separate file per layer; reuses entityManager surface)
- [x] No security issues introduced (allowlist-based validation, RFC 6901 escaping for paths)
- [x] No silent failures — soft conditions surface as warnings; hard failures return typed errors
- [x] No debug code left behind
