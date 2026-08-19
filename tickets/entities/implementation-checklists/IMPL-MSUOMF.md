---
id: IMPL-MSUOMF
type: implementation-checklist
title: 'Implementation: Consolidate cardinality analyzers; stop swallowing CountRelations errors'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code (behaviour-pinning tests written BEFORE the refactor and passing against the old code: `TestCheckCardinality_OrderingAndLabels`, `_BoundEdgeCases`, `_MultipleSourceTypes`; error-policy test after: `TestCheckCardinality_CountErrorFailsLoudly`)
- [x] Integration tests written (existing `internal/cli` tests exercise the commands end-to-end; manual CLI verification below)
- [x] Happy path implemented (`cardinalitySpec` + single `checkCardinality` + `countRelations` in `internal/analysis/analysis.go`)
- [x] Edge cases from planning handled (max=0 enforced, min=0 skipped, multi-type From ordering, inverse label on incoming, scope-before-count preserved)
- [x] Error handling in place (the ticket's core fix: `CountRelations` errors propagate; `CheckCardinality`/`AnalyzeAll` return error; all three CLI surfaces fail the command instead of fabricating violations)

## Test Quality

- [x] Using fixture builders or factories for test data (existing `addEntity`/`addRelation`/`newServiceWith` helpers; `failingCountStore` wraps a seeded memstore)
- [x] No hardcoded values in assertions when object is in scope (full expected `CardinalityViolation` structs compared, not ad-hoc fields)
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded (error test asserts `errors.Is` against the injected sentinel plus the id/relation context)
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

Built `cmd/rela` and ran against the live tickets/ project (2026-08-19): `rela
analyze cardinality`, `rela validate --check cardinality`, and `rela analyze
all` all report "All cardinality constraints satisfied" (exit 0) — identical to
the pre-refactor binary on the same data. AC1/AC2 verified by the pinning tests
(written first, unchanged by the refactor); AC3 by
`TestCheckCardinality_CountErrorFailsLoudly` (wrapped error names entity +
relation, zero violations returned, AnalyzeAll propagates); AC4 by the
compiler-enforced signature change + green `internal/cli` tests.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced (no world/pointer hook added — the spec is shaped for TKT-9KZGJO but carries no world field)
- [x] No silent failures (the change REMOVES a silent failure; remaining `collectEntities` under-count logging is documented out-of-scope in PLAN-KJ76OT)
- [x] No debug code left behind

## Quality Gates (run 2026-08-19)

- `go build ./...` + `go test ./...` — all pass
- `just arch-lint` — OK; `just plimsoll` — clean
- `just lint` — 0 issues (after dropping a now-unused nolint:prealloc directive)
- `just coverage-check` — floors + total (77.1% >= 65%) satisfied; internal/analysis 76.9%
