---
id: IMPL-6T34MV
type: implementation-checklist
title: 'Implementation: Read-only graph access in Lua migration steps: rela.get_entity and rela.get_relations'
started: "2026-10-13"
completed: "2026-10-13"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code — `internal/datamigration/luareads_test.go`
- [x] Integration tests written (test full flow, not just units) — every test
goes through `Runner.Run` (parse, step, write-back); the classification and
snapshot tests run on memstore, fsstore, sqlitestore and pgstore
- [x] Happy path implemented — `luareads.go`, installed in `luaStep.Run`
- [x] Edge cases from planning handled — missing id, named face, missing face,
ambiguous bare id, unknown type, invalid entity, `entity: "*"`,
`get_relations()` with no argument
- [x] Error handling in place (errors surfaced, not swallowed) — bad
arguments and store errors raise and fail the step; `get_relations` does not
return a partial list on a store error

## Test Quality

- [x] Using fixture builders or factories for test data — `seedInto`,
`readsFixture`, `readBackends`
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

- Without the `installGraphReads` call all nine `TestLuaReads_*` tests fail
(`rela` is nil). With a mutant `forEachEntity` that writes each patch as it
goes, `TestLuaReads_SeePreStepStateInEitherOrder` fails on every backend (`TSK-2
saw TSK-1=one!`), so the snapshot rule is pinned.
- pg leg run locally with `RELA_TEST_DATABASE_URL=postgres:///rela_test?host=/tmp`:
all pass. CI's Postgres job now runs `TestLuaReads_` with
`RELA_TEST_DATABASE_REQUIRED=1`.
- CLI end-to-end on an fs project (task, ncr, `gaat_over`): baseline, add
required `soort` enum, `rela migrate gen`, replace the drafted `set_default`
with a `lua:` step using the documented script. Dry run: `would change 2
record(s)`. Apply: `changed 2 record(s)`, validation `2 before, 0 after`;
TSK-001 (gaat_over NCR-001) got `corrective_action`, TSK-002 (gaat_over TSK-001)
got `general`. Second apply: nothing to run.

## Quality

- [x] Code follows project patterns (check similar code) — consumer-side
`graphReader` interface; argument checks mirror `internal/lua` `relationQuery`;
`entity.ParseAddress` instead of a bare Ref literal
- [x] Checked for DRY opportunities — `readEntityToTable` reuses
`entityToTable`; `internal/lua` helpers not imported by design (arch rule)
- [x] No security issues introduced — no write handle, same sandbox libraries
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
