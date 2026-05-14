---
id: IMPL-Z1O7
type: implementation-checklist
title: 'Implementation: Delegate wsEntityManager to entitymanager.Manager (wire Manager into production)'
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

- [x] ~~Feature manually tested end-to-end~~ (N/A: internal refactor — verified by running full `just ci` and 14 workspace test files unchanged)
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

All 11 acceptance criteria from PLAN-K0RQ are met:

- AC1: `internal/workspace.Workspace` holds `*entitymanager.Manager`; `New()` and `NewForTest()` both call `buildManager()` AFTER store assignment (workspace.go:189-191, workspace.go:266-269). `entitymanager.New` callers in workspace are only those two.
- AC2: `wsEntityManager` methods are all single-line forwarders (manager.go).
- AC3: `grep "func (w \*Workspace) createEntity\|updateEntity\|deleteEntity\|createEntityCore\|createRelation\|updateRelation\|deleteRelation"` returns no matches in `internal/workspace/*.go`.
- AC4: `internal/workspace/autocascade_host.go` deleted.
- AC5: `grep "runner.*autocascade.Runner\|autocascade.New"` in workspace returns only the buildManager() construction (Manager now owns the runner).
- AC6: `errors.Is(workspace.ErrHasRelations, entitymanager.ErrHasRelations) == true` via aliased var in workspace/errors.go.
- AC7: Manager partitions hard vs soft validation errors in CreateEntity/UpdateEntity/createCore. `validation_softening_test.go` passes unchanged.
- AC8: `entitymanager.partitionValidationErrors` exists at `internal/entitymanager/validation.go`. workspace's local copy removed.
- AC9: New tests `TestCreate_SoftValidationProducesWarning`, `TestCreate_HardValidationStillAborts`, `TestUpdate_SoftValidationProducesWarning` pin DEC-HWZHA behavior.
- AC10/AC11: `just ci` green (lint, arch-lint, race tests, coverage, docs).

**LOC summary:**

- workspace.go: 1439 → 911 (-528)
- workspace/manager.go: 171 → 64 (-107)
- workspace/autocascade_host.go: 146 → 0 (deleted)
- workspace/errors.go: 88 → 19 (-69, aliased to entitymanager)
- workspace/wsscriptrunner.go: 0 → 31 (new adapter)
- entitymanager/validation.go: 0 → 62 (new, DEC-HWZHA port)

Net deletion: ~750 LOC of dead code from workspace.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
