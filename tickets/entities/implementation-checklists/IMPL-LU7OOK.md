---
id: IMPL-LU7OOK
type: implementation-checklist
title: 'Implementation: Form relation direction: infer from schema, require it when self-referencing (drop the implicit outgoing default)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] Integration tests written (test full flow, not just units)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] Error handling in place (errors surfaced, not swallowed)

Files changed:

| File | Change |
| --- | --- |
| `internal/dataentryconfig/direction.go` | NEW — `InferDirection`, the single shared rule (`Resolved`/`Ambiguous`/`NoSide`) |
| `internal/dataentryconfig/validate.go` | Ambiguous bindings error; `validateFormRelationSide` uses the inferred direction |
| `internal/dataentry/api_v1.go` | `resolveRelationWidgets` → `resolveFormRelations`, now also fills direction before serving |
| `internal/migration/form_relation_direction.go` | NEW — writes explicit direction for unambiguous bindings (flat + wizard steps) |
| `internal/migration/dataentry_cleanup.go` | Removed `isRedundantDirection` (the strip that started this) |
| `internal/projectsetup/migrate_validate_roundtrip_test.go` | NEW — general migrate→validate guard |
| `docs/data-entry.md` | "How `direction` is resolved" + upgrade note |
| `tickets/`, `prototypes/*` data-entry.yaml | Migrated (26 auto + 8 hand-resolved in tickets/, 8 in prototypes) |

## Test Quality

- [x] Using fixture builders or factories for test data
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

Table-driven subtests throughout, shared metamodel fixtures
(`directionTestMetamodel`, `inverseTestMetamodel`).

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

Both new regression tests were confirmed to FAIL against the pre-fix code (via
`git stash`) and pass after — they are real regression tests, not tautologies.

End-to-end on a scratch project (task/project/self-referencing `depends-on`):

```
$ rela migrate
  ✓ form-relation-direction: Write explicit direction: on form relations that omit it
# from-side binding gained `direction: outgoing`
# to-side binding gained `direction: incoming`
# self-referencing `depends-on` left untouched

$ rela validate
  ✗ form "edit_task": relation[1] needs an explicit `direction:` — entity type
    "task" is both a from and a to of relation "depends-on", so outgoing and
    incoming are both valid and mean opposite things
```

On the real `tickets/data-entry.yaml`: 26 of 34 bindings auto-filled, 8
self-referencing listed by form + relation. After hand-resolving those 8 to
`outgoing` (preserving today's runtime behavior), the project validates clean
and a second `rela migrate` reports "No migrations needed" (idempotent).

AC verification:

| AC | Result | Evidence |
| --- | --- | --- |
| 1. from-side infers outgoing | PASS | `TestResolveFormRelations_Direction/from-side...` |
| 2. to-side infers incoming | PASS | `TestValidateConfig_FormRelationToSide_InfersIncoming`, `TestResolveFormRelations_Direction/to-side...` |
| 3. self-referencing errors | PASS | `TestValidateConfig_FormRelationSelfReferencing_RequiresDirection` |
| 4. explicit preserved | PASS | `..._ExplicitDirectionOK`, `TestFormRelationDirectionMigration/explicit_direction_is_preserved` |
| 5. migrate fills unambiguous, skips ambiguous | PASS | `TestFormRelationDirectionMigration` (7 subtests incl. wizard steps) |
| 6. migrate never invalidates | PASS | `TestMigrateThenValidate_RoundTrip` |
| 7. cleanup no longer strips direction | PASS | `TestDataEntryCleanupMigration_PreservesIncomingDirection` |

Pre-existing unrelated failure: `prototypes/data-entry/catalog` has an invalid
widget `"search"`; confirmed present before this change via `git stash`.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — one `InferDirection` shared by validation,
the server and the migration rather than three copies of the side-check
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
