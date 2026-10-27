---
id: IMPL-QV0W03
type: implementation-checklist
title: 'Implementation: Roll-up properties: read-time aggregates over related entities'
started: "2026-10-27"
completed: "2026-10-27"
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

- AC1: `internal/rollup/rollup_test.go` loads the brief's three roll-ups and
rejects an unknown relation, unknown property, enum value, pick type mismatch
and a non-date latest_by; `TestCompile_RejectsChain` refuses a chain.
`metamodel/rollup_test.go` covers the shape rules. `appbuild` schema check
refuses an automation condition and a validation that read a roll-up.
`entitymanager/rollup_test.go`: create and patch are refused, a whole-entity
update that echoes the value keeps the stored row.
- AC2: `storetest.RunRollupTests` passes on memstore, fsstore, sqlitestore and
pgstore (local PostgreSQL), including time values parsed from YAML.
`dataentry/rollups_test.go` checks GET, list (`rollups=1` and `list_id`), view
entry, view table section and the PATCH response; `mcp/rollups_test.go` checks
show_entity.
- AC3: `TestRollups_ReadSurfacesPerPrincipal` runs three principals: full
read (3, gefaald), a principal for whom one bewijs is hidden and `resultaat` is
invisible (2, absent), and one for whom `status` is invisible (0, gefaald).
`visibility/rollup_test.go` pins the same rules on the fold.
- AC4: `TestQueryBudget_ListRollupsAreSizeIndependent` pins 4 extra reads for
two roll-up columns at 10 and 50 rows on memstore; the SQL backends answer one
`Rollup` call per column (compile-time `store.Rollupper` assertions).
- Manual: a fs project under `.ignored/e2e` with the brief's schema. `rela show
MAAT-1` printed `bewijs_aantal: 2` (stored 99 not shown) and
`laatste_testresultaat: gefaald`; `rela list maatregel -o json` the same;
`--sort bewijs_aantal` is refused. This run found that the fs store holds an
unquoted YAML date as a time value, which the Go fold ignored; fixed with
`store.RollupText` and a conformance case on all four backends. `rela-server`
was not started: the worktree has no built SPA. The handler tests drive the same
HTTP handlers.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] ~~No silent failures (errors logged AND returned)~~ (N/A as worded: a roll-up that fails to compute is logged with slog.Warn and left absent, so one broken aggregate does not fail the entity read; schema errors fail the load)
- [x] No debug code left behind
