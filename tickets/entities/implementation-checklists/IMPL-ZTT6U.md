---
id: IMPL-ZTT6U
type: implementation-checklist
title: 'Implementation: MCP update_entity should support deleting properties'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code (12 new tests in `tools_test.go` + `convert_test.go`)
- [x] Integration tests written — handler tests round-trip through the workspace + memstore (full update flow, not just helper functions)
- [x] Happy path implemented (`null` deletes via `delete()` in the merge loop)
- [x] Edge cases from planning handled (empty string no-op, delete-only call survives guard, JSON-string fallback, unknown property rejected, no-op on absent property, mixed set+unset)
- [x] Error handling in place — existing error paths (entity not found, no updates specified, unknown properties) preserved; nil values flow cleanly through validation

## Test Quality

- [x] Using fixture builders or factories for test data (`testutil.EntityFor` via `makeTestServer`)
- [x] No hardcoded values in assertions when object is in scope (compare against original `before.GetString("status")`)
- [x] Only specifying values that matter for the test
- [x] ~~Interpolated values constructed from objects, not hardcoded~~ (N/A: no interpolation in these tests)
- [x] Property comparisons use original object, not hardcoded strings (where applicable; some tests assert specific status values like `"rejected"` because that IS the value being asserted)

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

Test results — all 21 update-entity / extract-properties tests pass:

```
=== RUN   TestExtractPropertiesAllowNil_PreservesNil               PASS
=== RUN   TestExtractPropertiesAllowNil_OnlyNilEntriesStillReturnsMap PASS
=== RUN   TestExtractPropertiesAllowNil_StringJSONNullDeletes      PASS
=== RUN   TestExtractPropertiesAllowNil_FiltersEmptyString         PASS
=== RUN   TestExtractPropertiesAllowNil_NoArg                      PASS
=== RUN   TestExtractPropertiesAllowNil_InvalidJSON                PASS
=== RUN   TestHandleUpdateEntity_DeletesPropertyOnNil              PASS  (AC 1)
=== RUN   TestHandleUpdateEntity_DeleteOnlyCallSurvivesGuard       PASS  (AC 7)
=== RUN   TestHandleUpdateEntity_DeleteAbsentPropertyIsNoOp        PASS  (AC 4)
=== RUN   TestHandleUpdateEntity_DeleteUnknownPropertyRejected     PASS  (AC 5)
=== RUN   TestHandleUpdateEntity_MixedSetAndUnset                  PASS  (AC 6)
=== RUN   TestHandleUpdateEntity_EmptyStringIsNoOp                 PASS  (AC 8)
=== RUN   TestHandleUpdateEntity_SetAndOverwriteStillWorks         PASS  (AC 3)
=== RUN   TestUpdateEntityToolDescriptionMentionsNullDelete        PASS  (AC 2)
=== RUN   TestExtractProperties_*                                  PASS  (regression)
=== RUN   TestHandleUpdateEntity_NoUpdates / NotFound              PASS  (regression)
```

End-to-end JSON-schema check — built a tiny program that constructs
`toolUpdateEntity()` and dumps the rendered schema:

- top-level `description` reads: `"Update an existing entity's properties or content. Set a property to null in 'properties' to remove it from the entity."`
- `properties` arg description reads: `"Properties to set or update. Set a property to null to remove it from the entity. Empty string is treated as no value (silently ignored)."`
- both descriptions are visible to clients (mcp-go renders both per `tools.go:590` and `tools.go:902`)

AC 9 (JSON-string with null) verified at the helper level via
`TestExtractPropertiesAllowNil_StringJSONNullDeletes`.

The `becomes`-automation-does-not-fire test from the plan was DROPPED: wiring an
automation engine into `makeTestServer` is more wiring than the value justifies,
and the contract is already covered by `internal/automation` tests of
`engine.go:190-196` (oldValue read via `GetString`, returns `""` for missing
keys). Documented behavior: deleting a property looks like "set to empty string"
to the automation engine — `becomes:<specific value>` won't fire.

`just lint` clean. `just test` passes (no regressions). `just coverage-check`
passes (72.9% total, all package floors met).

## Quality

- [x] Code follows project patterns — new helpers mirror the shape of the existing `extractProperties`; same nil-handling-via-shared-parser approach
- [x] No security issues introduced — property names still allowlist-validated; nil values bypass type validation correctly (no value to validate)
- [x] No silent failures — error paths preserved; no new logging needed
- [x] No debug code left behind
