---
id: PLAN-4HN2C
type: planning-checklist
title: 'Planning: MCP update_entity should support deleting properties'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

IN: Add a way for MCP clients to remove a property via `update_entity`. Update
the tool's input schema/description so AI assistants discover it. Tests cover
the new mechanism. The `null`-value approach is chosen (see Approach).

OUT: Auto-pruning on every entity write. New `delete_property` tool. Bulk
schema-migration tooling. Touching `unset` for relations. Changing how empty
string `""` is treated.

**Acceptance Criteria:**

1. `update_entity` with `properties: {"foo": null}` removes `foo` from the entity returned by the store after the update. → Test: write entity with `foo=bar`, call handler with `{"foo": null}`, fetch via `st.GetEntity`, assert no `foo` key.
2. The MCP tool description for `update_entity` documents the null-deletion contract on BOTH the top-level tool description and the `properties` arg description (mcp-go renders both). → Test: assert the description strings on the constructed tool include "null".
3. Existing set/overwrite semantics unchanged. → Existing `TestHandleUpdateEntity_*` tests still pass; new `TestHandleUpdateEntity_SetAndOverwriteStillWorks` covers the positive path explicitly.
4. `update_entity` with `{"foo": null}` on an entity that has no `foo` is a no-op (no error). → Test: handler returns success; entity unchanged.
5. Property-name validation: unsetting requires the property name to exist in the metamodel for the entity type, same as setting. → Test: `{"unknown_prop": null}` returns the existing "unknown properties" error.
6. Mixed set+unset in one call works: `{"foo": null, "bar": "baz"}` removes `foo` and sets `bar=baz`. → Dedicated test.
7. **Delete-only call survives the "no updates specified" guard.** `{"properties": {"foo": null}}` with no other args reaches the update path (does NOT trip the empty-properties error at `tools_entity.go:208`). → Dedicated test.
8. Empty string remains a silent no-op: `{"foo": ""}` does not delete or change `foo`. → Dedicated test (preserves create-path consistency).
9. JSON-string-encoded properties also support null-as-delete: `properties` arg as a JSON string `"{\"foo\": null}"` deletes `foo`. → Test on the helper level.

## Research

- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Existing Solutions:**

- The storage layer already supports property deletion. `internal/workspace/manager.go:67-70` rebuilds `updated.Properties` from `e.Properties` on every `UpdateEntity`, so any key not present in the incoming map is dropped on disk. The MCP handler is the one place that hides this by doing an additive merge.
- The `null`-as-delete pattern is the JSON-Merge-Patch convention (RFC 7396) and matches the issue's primary suggestion.
- mcp-go v0.43.2 (`tools.go:590` and `tools.go:902`) renders both the top-level tool description and per-property descriptions in the JSON schema sent to clients.
- Concept: `mcp-api` (stable) — this ticket extends it.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

Use `null` in the `properties` map as the delete sentinel.

Three coordinated changes in `internal/mcp`:

1. `tools_helpers.go` — split `extractProperties`:
   - Refactor: extract a shared `parsePropertiesArg(request) (map[string]interface{}, bool)` that handles the `map[string]interface{}` case AND the JSON-encoded-string fallback (currently inlined in `extractProperties`).
   - Keep `extractProperties` (filters both `nil` and `""`) — used by create paths.
   - Add `extractPropertiesAllowNil` — filters only `""`. **Returns the map even if it contains only nil entries** (so `len()` reflects the delete intent and the "no updates specified" guard at `tools_entity.go:208` does not reject delete-only calls). Returns `nil` only when the input arg is missing/malformed.
2. `tools_entity.go` — `handleUpdateEntity`:
   - Switch to `extractPropertiesAllowNil`.
   - Validation step (`validatePropertyNames`) runs unchanged; nil values pass through, but property names are still allowlist-checked against `entityDef.Properties`. This implicitly rejects `{"id": null}` since `id` is not a metamodel property in any type (verified via grep on `tickets/metamodel.yaml`).
   - In the merge loop: when `v == nil`, `delete(e.Properties, k)`; otherwise `e.Properties[k] = v` as today.
3. `tools.go` — extend `update_entity` tool description:
   - Top-level: add "Set a property to null to remove it from the entity. Empty string is treated as no value (silently ignored)."
   - `properties` arg description: same line.

Storage-layer change: none. Workspace already does the right thing.

**Alternatives considered:**

- Separate `unset: ["foo"]` array. Rejected: doubles the surface, AI clients learn two shapes for one op, and JSON-Merge-Patch null is the convention.
- Auto-pruning on every write. Rejected: out of scope per issue; future migrate subcommand.
- New `delete_property` MCP tool. Rejected: overlaps `update_entity`.
- Treating empty string `""` as delete. Rejected: would break `extractProperties` for create paths (empty form fields would have to be tolerated separately) and the helpers stay symmetric only if we don't touch the empty-string behavior. Documented as a silent no-op instead (AC 8).

**Files to modify:**

- `internal/mcp/tools_helpers.go` — refactor + new `extractPropertiesAllowNil`.
- `internal/mcp/tools_entity.go` — `handleUpdateEntity` uses the new helper and treats nil as delete.
- `internal/mcp/tools.go` — update description on `update_entity` (tool desc + `properties` arg desc).
- `internal/mcp/tools_test.go` — new tests (see Test Plan).
- `internal/mcp/convert_test.go` — keep existing `extractProperties` tests; add tests for `extractPropertiesAllowNil`.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

- The `properties` map comes from the MCP client (AI assistant). Property names are still validated against the metamodel allowlist (`validatePropertyNames`). Nil values bypass type validation (no value to validate), which is correct — we're deleting the key.
- `id` cannot be deleted because it is not declared as a metamodel property in any entity type (verified). If a future entity type adds `id` as a property, validation would let it through; left as a tracked assumption rather than a hardcoded guard.

**Security-Sensitive Operations:**

- Property deletion is a write to the entity file. Same trust model as the existing set/overwrite path; no new privilege escalation surface. Same automation triggers fire on update (see Edge Cases). No new error-message content.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**

| AC | Test |
| --- | --- |
| 1 | `TestHandleUpdateEntity_DeletesPropertyOnNil` — entity with prop, send `{"prop": nil}`, assert prop absent in store after update. |
| 2 | `TestUpdateEntityToolDescriptionMentionsNullDelete` — assert both tool desc and `properties` arg desc strings contain "null". |
| 3 | `TestHandleUpdateEntity_SetAndOverwriteStillWorks` — sanity check positive path. |
| 4 | `TestHandleUpdateEntity_DeleteAbsentPropertyIsNoOp` — entity without `foo`, send `{"foo": nil}`, assert success and entity unchanged. |
| 5 | `TestHandleUpdateEntity_DeleteUnknownPropertyRejected` — unknown prop name with nil value still returns "unknown properties" error. |
| 6 | `TestHandleUpdateEntity_MixedSetAndUnset` — `{"a": nil, "b": "x"}`, assert `a` removed and `b=x`. |
| 7 | `TestHandleUpdateEntity_DeleteOnlyCallSurvivesGuard` — `{"foo": null}` as the sole property arg does NOT trigger "no updates specified". |
| 8 | `TestHandleUpdateEntity_EmptyStringIsNoOp` — `{"foo": ""}` leaves an existing `foo=bar` untouched. |
| 9 | `TestExtractPropertiesAllowNil_StringJSONNullDeletes` — helper-level test: arg passed as JSON string containing a `null` value yields a map with the nil entry preserved. |
| - | `TestHandleUpdateEntity_DeleteDoesNotFireBecomesAutomation` — delete a property with a `becomes:<value>` automation; assert the automation does NOT fire (newValue is "" not the trigger value). Documents the contract. |

All tests use `memstore.New()` (existing pattern via `makeTestServer`); on-disk
persistence is owned by `internal/markdown` + workspace and has its own
coverage.

**Edge Cases:**

- `{"foo": ""}` — empty string. Silent no-op (filtered out). AC 8.
- `{"foo": nil}` on an entity where `foo` was never set — no-op. AC 4.
- Property has a `becomes:<value>` automation — deletion produces (oldValue=<previous>, newValue="") per `internal/automation/engine.go:190-196`. Automations with `becomes:<specific value>` won't fire (newValue="" doesn't match). Documented as the contract.
- Required-by-metamodel property is deleted — entity becomes invalid; surfaced by `analyze_validations`/`analyze_cardinality`, not gated here. Tool description should not promise validation.
- Concurrent updates — same locking as today's set path; no new concurrency surface.
- `id` deletion — `validatePropertyNames` rejects it because `id` is not a declared metamodel property in any entity type.

**Negative Tests:**

- Unknown property name: existing validation error returned (AC 5).
- Entity not found: existing `TestHandleUpdateEntity_NotFound` covers it.
- Empty `properties` map AND empty content: existing "no updates specified" error still fires (`{}` properties → `len()==0` → guard trips).

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

- **Risk:** New helper accidentally collapses to nil when only nil entries are present, killing the delete-only path. **Mitigation:** explicit AC 7 test; helper documented to preserve nil entries and count them toward `len()`.
- **Risk:** JSON-string fallback diverges between the two helpers. **Mitigation:** shared `parsePropertiesArg` extracts the parsing; both helpers wrap it.
- **Risk:** AI assistants don't discover the new capability. **Mitigation:** documented in BOTH the top-level tool description and the `properties` arg description (mcp-go renders both).
- **Risk:** Deleting a required property leaves the project in a validation-failing state. **Mitigation:** explicit non-goal — `analyze_validations`/`analyze_cardinality` already catch this.

**Effort:** s

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**

- [x] User guide / reference docs — the MCP tool description IS the reference doc surface; updated in code.
- [x] ~~CLAUDE.md~~ (N/A: no change needed)
- [x] ~~README.md~~ (N/A: no project-level changes)
- [x] API docs — MCP tool description = API doc; updated.

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** RR-3H7IW (critical, addressed), RR-8N1CZ
(significant, addressed), RR-RUI15 (significant, addressed), RR-1JHC2 (minor,
addressed), RR-CDGL1 (minor, addressed), RR-OXOIT (minor, addressed), RR-8S190
(nit, addressed).
