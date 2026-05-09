---
id: PLAN-MXQKI
type: planning-checklist
title: 'Planning: Extend PATCH /entities/{id} relations to carry per-edge meta + content'
status: in-progress
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

**In scope:**
- Wire format extension: `PATCH /api/v1/{plural}/{id}` accepts `relations: map[string]V1RelationsUpdate` where `V1RelationsUpdate = {data: [{type, id, meta?, meta_unset?, content?}]}`. Coexists with the legacy `relations: map[string][]string` form via custom JSON unmarshal that distinguishes the two shapes.
- `reconcileOutgoingRelations` extended (or paralleled) to accept the new shape and, per edge, decide create vs upsert vs delete. Upsert calls `entityManager.UpdateRelation` with the merged meta + content.
- Per-edge meta upsert: `meta` keys merge into existing properties; `meta_unset` removes keys.
- Per-edge content upsert: `content: "string"` replaces (including empty string clears); absent leaves alone.
- **Validation policy per DEC-HWZHA**: hard 400 for malformed wire format, hard 422 for structural impossibilities (unknown relation type, content on non-content-bearing relation type), 200 + warnings for everything else (target type mismatch, missing target entity, unknown meta keys, required-meta unset, meta type mismatches).
- Validation runs before any writes (validate-then-write, not write-then-validate). Critically, the validation phase for the relation reconcile runs **BEFORE** the entity is updated, so a relation-shape error doesn't leave the entity half-written.
- `data` field is required when wrapper appears (`{"tagged":{}}` → 400).
- **Value-based no-op suppression** at the per-edge level: post-merge state byte-equals current state → no write, no SSE event. Achieved with `reflect.DeepEqual` on the (final-properties-map, final-content) tuple after merging incoming meta and resolving meta_unset.
- Backwards compat for legacy IDs-only shape: continue to work unchanged. Both shapes follow the same validation policy; only modern returns warnings inline.
- **Extending `entitymanager.RelationOptions` with `MetaUnset []string`** and changing `Content string` → `Content *string` so meta-clear and content-clear semantics are expressible. Both `workspace.updateRelation` and store-level callers honor the new fields. All existing callers audited and updated (enumerated below in Approach).
- New response field: `warnings: []` carrying `{code, path, detail}` objects. Code values match the corresponding `analyze_*` finding codes.
- OpenAPI schema regenerated for the new shape and the new response field.
- Tests: integration tests against the existing test harness, covering each acceptance criterion below plus negative tests.

**Out of scope:**
- Frontend wiring of `RelationPicker` / `RelationCards` to the new shape — TKT-B9SXH / TKT-18JS6 will consume the API. We do not add TypeScript types in this PR; the consumer tickets add them when they wire up.
- Symmetric / inverse propagation of per-edge meta — current `writeRelation` does not propagate at all today (workspace.go:416). Separate ticket if needed.
- Multi-write atomicity (entity update + relation reconcile as a single store transaction). Store has no transaction primitive (FEAT-CO4YP). With validation-first ordering (see Approach), the only failure window is mid-write-loop, which is rare and irrecoverable. Documented in api-reference.md.
- Migrating callers off the legacy IDs-only shape (deprecation deferred).
- Auditing/redesigning workspace.go's symmetric/inverse handling — independent gap.
- Retroactively softening hard 422s on existing endpoints — per DEC-HWZHA migration scope, that's separate-ticket work.

**Acceptance Criteria (under DEC-HWZHA validation policy):**

### Behavioral

1. **AC1 — Add new edge with meta**: PATCH with `relations: {tagged: {data: [{type: label, id: L-001, meta: {weight: 5}}]}}` creates the edge with `weight=5`. **Test**: assert relation persisted with the meta value.
2. **AC2 — Upsert meta on existing edge**: PATCH same target with `meta: {weight: 7}` updates the existing relation's meta (no duplicate created). **Test**: re-PATCH, assert weight=7, assert single relation.
3. **AC3 — Meta unset clears keys**: PATCH with `meta_unset: ["weight"]` removes the key from existing meta. **Test**: existing meta has weight; PATCH; assert weight is gone.
4. **AC4 — Per-edge content upsert**: PATCH with `content: "edge body"` on a relation type with `content: true` writes the body. Subsequent PATCH with `content: ""` clears. **Test**: assert relation file body matches.
5. **AC5 — `data: []` removes all of that type**: PATCH with `relations: {tagged: {data: []}}` removes every existing tagged edge. **Test**: assert zero outgoing edges of that type.
6. **AC6 — Replacement semantics**: existing edges A, B, C; PATCH with `data: [{A}, {D}]`; result: A kept, D added, B and C removed. **Test**: assert exact set after PATCH.
7. **AC7 — Absent relation type leaves alone**: existing tagged edges; PATCH with no `relations.tagged` key; result: existing edges unchanged. **Test**: assert no diff.
8. **AC15 — Backwards compat with IDs-only shape**: PATCH with legacy `relations: {tagged: ["L-001", "L-002"]}` continues to work; meta-less edges created; same warnings as modern shape would emit (target-existence checks, etc.). **Test**: assert both shapes produce equivalent results when both succeed.
9. **AC16 — Combined PATCH**: PATCH with `properties`, `content`, AND `relations` in one body — all applied; entity event fires once. **Test**: assert all three persisted.
10. **AC17 — Validation failure leaves entity AND relations untouched**: PATCH with valid properties + a relation entry that fails validation phase (e.g. unknown relation type → structural 422); entity is NOT updated, no relation writes happen. **Test**: assert pre-PATCH state preserved on disk and in memory after the request returns 422. *(Achievable because validation runs BEFORE entity update — see Approach Layer 3.)*

### Hard 400 — wire-format errors

8. **AC8 — `{"tagged": {}}` returns 400**: data-loss footgun mitigation. **Test**: assert HTTP 400 with structured error pointer.
9. **AC18 — Legacy + new shape mixed in one body returns 400**: e.g. one relation type uses `{data: [...]}` and another uses `[...]` — reject as ambiguous. **Test**: assert HTTP 400 with stable error code `shape_mixed`. Detail is generic ("request mixes legacy and JSON:API relation shapes; pick one"), does NOT name a specific key (Go map iteration is non-deterministic — RR-FOLOX).
10. **AC19 — Sibling-key rejection**: `{"tagged": {"datas":[]}}` returns 400 (only `data` is allowed inside the modern wrapper). Custom UnmarshalJSON enforces this with `DisallowUnknownFields`-style check.
20a. **AC20a — Non-string element in meta_unset returns 400**: `meta_unset:
["x", null]` and `meta_unset: ["x", 5]` both 400 at unmarshal time with index
pointer. 20b. **AC20b — `data` is non-array in modern wrapper returns 400**:
`{"tagged": {"data": "L-001"}}` returns 400. 20c. **AC20c — `data: null` returns
400**: same as missing `data` key (RR-UZ8LX). Stable error code `data_required`.

### Hard 422 — structural impossibilities

9. **AC9 — Unknown relation type returns 422**: PATCH with `relations: {nonexistent: {data: []}}` returns 422 with code `unknown_relation_type`. Justification: there is no defined storage location for a relation file of an unknown type. **Test**: assert HTTP 422.
10. **AC13 — Content on non-content-bearing type returns 422**: relation type without `content: true` and PATCH carries `content`. The disk format can't hold a body for that type. **Test**: assert HTTP 422 with code `content_not_supported`.

### 200 + warnings — soft conditions

10. **AC10 — Target type mismatch surfaces warning**: PATCH with target ID whose actual type doesn't match `data[*].type`. Edge IS written; response includes `warnings: [{code: "target_type_mismatch", path: "/relations/.../data/0", detail: "..."}]`. **Test**: assert HTTP 200, edge persisted, warning in response.
11. **AC11 — Target ID doesn't exist surfaces warning**: PATCH with target ID that's not in the graph. Edge IS written (the relation file references the nonexistent target — `analyze_orphans` will flag it). Response includes `warnings: [{code: "target_not_found", ...}]`. **Test**: assert HTTP 200, edge persisted, warning in response.
12. **AC12 — Unknown meta key surfaces warning**: relation type has closed schema; meta has unknown key. Edge IS written with the extra meta. Response includes `warnings: [{code: "unknown_meta_key", path: "/.../meta/X", detail: "..."}]`. **Test**: assert HTTP 200, meta key persisted, warning in response.
12b. **AC12b — Required meta unset surfaces warning**: relation type has
required meta property; PATCH creates an edge or unsets the property leaving it
absent. Edge IS written. Response includes `warnings: [{code:
"required_meta_unset", ...}]`. **Test**: assert HTTP 200, edge persisted without
the required key, warning in response. 12c. **AC12c — Meta type mismatch
surfaces warning**: PATCH with meta value whose type doesn't match declared
property type. Edge IS written with the wrong-typed value. Response includes
`warnings: [{code: "meta_type_mismatch", ...}]`. **Test**: assert HTTP 200.

### No-op + atomicity

14. **AC14 — Value-based no-op suppression**: PATCH with desired state that, after merge resolution, byte-equals current state; assert zero relation writes (verified via Store event subscriber counter) and zero SSE events. Includes the case where the same `meta: {weight: 5}` is re-sent on every save (auto-save's primary use case — RR-M3LWM).
14b. **AC14b — Shape-based no-op for absent fields**: when a desired edge has no
`meta`, no `meta_unset`, no `content` AND already exists, no write fires. Strict
subset of AC14.

### Documentation

20. **AC20 — OpenAPI schema describes both shapes plus warnings field**: regenerated `openapi.yaml` shows the new wire shape with `data` array, per-edge fields, and the response `warnings` array.

## Research

- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Existing Solutions:**

- **JSON:API §9 (Resource Identifier Objects)**: provides the wire shape (`{type, id, meta?}` arrays under `data`). We borrow the shape, not the full envelope (response stays rela's flat format with the new `warnings` field). No JSON:API library used; the surface is small and a bespoke `UnmarshalJSON` is clearer than pulling in a dependency.
- **Existing in-tree patterns:**
  - `internal/dataentry/relations.go:45` `reconcileOutgoingRelations` — current set-semantic reconciler. Extension target.
  - `internal/dataentry/api_v1.go:504` `handleV1UpdateEntity` — current PATCH handler. Wire format extension target.
  - `internal/entitymanager/entitymanager.go:71` `RelationOptions{Properties, Content}` — already supports per-edge data on Create/UpdateRelation; this PR extends the type with `MetaUnset` + `*string Content`.
  - `internal/workspace/workspace.go:1405` `updateRelation` — backend impl. Currently merges Properties (no clear), conditionally overwrites Content (no clear-to-empty). This PR fixes both AND updates the godoc to document merge-then-unset semantics (RR-4G6V0).
  - `internal/store/store.go:188` `RelationWriter` — store interface. `UpdateRelation(ctx, from, type, to, RelationData)` takes a full `RelationData{Properties, Content}` — the store-level call is full-replacement. The merge happens in `workspace.updateRelation`, which this PR adjusts.
  - `internal/dataentry/relations.go:11-31` `relationError` type — existing structured error with `Op`/`Reason`/`Target`/`RelType` codes. Reuse for hard-422 codes; warnings get a parallel `Warning` struct with the same shape minus `Op`.
- **Prior art (closed PR #648)**: explored a parallel `Manager` struct with full WithTx orchestration. Doesn't apply on current develop because (a) the `EntityManager` interface already exists with a different shape and (b) the store has no transaction primitive. Wire format design from that PR is reusable. Notably, the closed PR had the SAME validation drift (hard 422 on soft conditions) that DEC-HWZHA explicitly addresses.
- **rela concepts:** `FEAT-jsjj` "Relation properties and content support" is the parent feature. `FEAT-CO4YP` "Pluggable store backends" constrains atomicity decisions. `DEC-HWZHA` "Validation policy for write APIs" governs the validation classes.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

**Layer 0 — `entitymanager.RelationOptions` extension**
(`internal/entitymanager/entitymanager.go`):

```go
// RelationOptions configure relation creation/update.
//
// On CreateRelation: Properties is the initial property map, MetaUnset is
// ignored (no existing values to clear), Content (if non-nil) sets the body.
//
// On UpdateRelation: Properties MERGES into the existing properties (this
// is the existing behavior — kept for back-compat). After the merge,
// MetaUnset deletes the named keys. Content (if non-nil) replaces the
// body, including with empty string ""; if nil, the existing body is
// untouched.
type RelationOptions struct {
    Properties map[string]interface{}
    MetaUnset  []string  // NEW: keys to clear after the merge (UpdateRelation only)
    Content    *string   // CHANGED: was string; pointer distinguishes leave-alone vs set-to-empty
}
```

`workspace.updateRelation` (workspace.go:1405) is updated:
- Godoc rewritten to document the merge-then-unset semantics (RR-4G6V0).
- After merging `opts.Properties`, iterate `opts.MetaUnset` and `delete()` each key. (Deleting a missing key is a Go no-op; that's the documented behavior — RR-D895N already addressed this.)
- Replace `if opts.Content != ""` with `if opts.Content != nil` and assign `*opts.Content`.

**Call-site enumeration** (RR-X5WVA — done up front instead of "during
implementation"):

| File:line | Current call | New call | Behavioral change? |
|---|---|---|---|
| `internal/dataentry/handlers_api.go:527` | `RelationOptions{}` | unchanged | no |
| `internal/dataentry/relations.go:113` | `RelationOptions{}` | unchanged | no |
| `internal/dataentry/api_v1.go:730` | `RelationOptions{Properties: meta}` | unchanged | no |
| `internal/dataentry/api_v1.go:772` | `RelationOptions{Properties: meta}` | unchanged | no |
| `internal/workspace/manager.go:138` | `RelationOptions{Properties, Content}` | propagate MetaUnset + Content *string | no |
| `internal/workspace/manager.go:147` | `RelationOptions{Properties, Content}` | propagate MetaUnset + Content *string | no |
| `internal/mcp/tools_relation.go:76` | `Content: request.GetString("content", "")` | `Content: nilIfEmpty(request.GetString("content", ""))` | **subtle**: today, `content: ""` from MCP leaves alone (treated as default). New behavior: still leaves alone (because `nilIfEmpty` returns nil for ""). MCP tool docs note: pass `null` or omit `content` to leave alone; the empty-string ambiguity is preserved by the helper. (RR-QOIXK) |
| `internal/cli/link.go:27` | `Content: ""` (no content arg) | `Content: nil` | no |
| `internal/lua/runtime.go:1373` | builds `Content: contentStr` | use `Content: &contentStr` if Lua set it, else nil | **subtle**: Lua `rela.update_relation(..., {content = ""})` previously left content alone; new behavior matches what users expect: empty string clears. Document in lua API reference. (RR-QOIXK) |
| `internal/entitymanager/entitymanagertest/panic.go` | stub | update signature to match | no |

The MCP and Lua semantic changes are deliberate and aligned with the new general
principle: empty string means empty string, nil means leave alone. Document both
in their respective references.

**Layer 1 — Wire format types** (`internal/dataentry/api_v1.go` + new test
file):

```go
// V1RelationsField is the top-level value for `relations` in a PATCH body.
// Custom JSON unmarshal accepts EITHER the legacy IDs-only shape
// (map[string][]string) OR the new JSON:API shape (map[string]V1RelationsUpdate).
type V1RelationsField struct {
    Legacy map[string][]string
    Modern map[string]V1RelationsUpdate
    // Mixing the two shapes in one body returns an error during unmarshal.
}

type V1RelationsUpdate struct {
    Data        []V1ResourceIdentifier
    DataPresent bool  // distinguishes {"tagged":{}} from {"tagged":{"data":[]}}
}

type V1ResourceIdentifier struct {
    Type      string
    ID        string
    Meta      map[string]interface{}
    MetaUnset []string
    Content   *string
}
```

**`UnmarshalJSON` state machine for `V1RelationsField`** (RR-0NTMQ):

1. Decode body into `map[string]json.RawMessage` first.
2. Track two flags: `sawLegacy`, `sawModern`. If both ever set, return `&shapeMixedError{}` (stable code `shape_mixed`, detail does NOT name a specific key — RR-FOLOX).
3. For each (relation-type, raw-value):
   - Trim leading whitespace from `raw` (handles BOM and structural whitespace).
   - Inspect first non-whitespace byte:
     - `[` → legacy IDs-only branch. Unmarshal as `[]string`; on error, return 400 with JSON pointer.
     - `{` → modern wrapper branch. See sub-state-machine below.
     - `n` (likely `null`) → check exact text matches `null`. If yes: 400 with code `relation_value_null` ("relation type cannot be null; use `data: []` to clear"). RR-UZ8LX.
     - Anything else (`"` for string, digit for number, `t`/`f` for bool, etc.) → 400 with code `relation_value_invalid`.
4. Sub-state-machine for modern wrapper `{...}`:
   - Decode into `map[string]json.RawMessage`.
   - Check sibling keys: only `data` allowed. Anything else → 400 code `unknown_field`.
   - If `data` key absent: `DataPresent=false`, `Data=nil`. (Caller logic emits 400 `data_required`.)
   - If `data` is JSON `null`: 400 code `data_required` (treat same as missing — RR-UZ8LX).
   - Otherwise: try to unmarshal `data` as `[]V1ResourceIdentifier`. If the value isn't an array (it's an object, scalar, etc.), 400 code `data_invalid_type`. AC20b.
5. Sub-unmarshal for `V1ResourceIdentifier`:
   - `meta_unset` must be `[]string` or absent. Each element must be a string. Non-string element → 400 with index pointer (RR-LGK1X / AC20a).
   - `meta: null` → treated as absent (`Meta = nil`).
   - `meta_unset: null` → treated as absent.
   - `content: null` → treated as absent (pointer remains nil).

**JSON pointer error format** (RR-YNWR7): all path components apply RFC 6901
escaping (`~` → `~0`, `/` → `~1`). New helper `jsonPointerEscape(s string)
string` in `internal/dataentry/`. Use for relation-type names and meta keys
whenever they appear in error pointers. The format is documented as "RFC 6901
JSON Pointer" in the api-reference.

**Layer 2 — Modern reconciler** (`internal/dataentry/relations.go`):

Add `reconcileOutgoingRelationsModern(ctx, entityID, desired
map[string]V1RelationsUpdate)`. Returns `(warnings []Warning, err error)`.
Legacy reconciler stays unchanged (and gets the same warning surface added — see
below).

The modern reconciler:
1. **Validation phase** (no writes):
   - Walk the desired map; for each entry classify via DEC-HWZHA buckets:
     - **400** issues already trapped at unmarshal time.
     - **422 structural**: relation type unknown? content on non-content-type? Return immediately with code.
     - **Soft conditions**: target existence, target type allowed, source type allowed, meta key closed-schema, required meta unset, meta type mismatch. Each yields a `Warning{code, path, detail}` appended to a local list.
2. **Diff phase**: per relation type, build `desiredByID` and `currentByType[relType]` (current outgoing edges of that type). Per desired edge:
   - Compute the post-merge `(finalProps, finalContent)` by reading current edge (if any), applying `Meta` merge, applying `MetaUnset` deletes, replacing `Content` if non-nil.
   - If in current AND `reflect.DeepEqual(currentProps, finalProps) && currentContent == finalContent` → skip (value-based no-op suppression — RR-M3LWM).
   - If in current → upsert via `UpdateRelation`.
   - If not in current → create via `CreateRelation` with the post-merge values.
For each current edge not in desired → delete via `DeleteRelation`.
3. **Write phase**: stage adds/upserts/removes; iterate. Each write is wrapped — failure produces `*relationError` with `Reason: "create_failed"|"update_failed"|"delete_failed"`. Still a 422 (structural — store said no), not a warning.

**Note on `reflect.DeepEqual` for value-based suppression**: JSON unmarshal into
`interface{}` always produces `float64` for numbers; the on-disk values reaching
the workspace come from the *same* JSON unmarshal in our case (relation files in
markdown frontmatter parse via YAML, but the stored map is loaded as
`map[string]interface{}` and round-trips through the same YAML decoder which
uses Go-native types). The "numeric normalization" worry from PR #648 was
overstated for the in-memory comparison case. Document this assumption in
api-reference.md; if it bites later, fall back to a small `valueEqual` helper in
`internal/model`.

**Layer 3 — Handler dispatch and ordering** (`internal/dataentry/api_v1.go:504`
`handleV1UpdateEntity`):

Replace `Relations map[string][]string` with `Relations V1RelationsField`. After
parsing:

```go
// Phase A: validate-only run on the relation reconciler. Returns warnings
// (informational) and err (hard 400/422). On err, return immediately —
// no writes anywhere.
warnings, err := a.validateRelationsRequest(r.Context(), entityID, req.Relations)
if err != nil {
    writeV1Error(w, r, ...)
    return
}

// Phase B: entity update. May emit its own validation 422 or 400.
if entityChanged {
    if _, err := a.entityManager.UpdateEntity(...); err != nil {
        writeV1Error(...)
        return
    }
}

// Phase C: relation writes (we already validated; only store-level failures possible here).
moreWarnings, err := a.applyRelationsRequest(r.Context(), entityID, req.Relations)
if err != nil {
    // Store error mid-loop; entity is already updated. This is the
    // documented atomicity gap — return 500 with detail.
    writeV1Error(w, r, http.StatusInternalServerError, ...)
    return
}

// Successful response includes BOTH validation-phase and write-phase warnings.
result.Warnings = append(warnings, moreWarnings...)
writeV1JSON(w, http.StatusOK, result)
```

This makes AC17 achievable (RR-CW1FK): a relation 400/422 returns *before*
`UpdateEntity` runs.

**Validation-phase ordering note** (RR-PZY1W): relation-validation runs FIRST.
Entity validation runs second (inside `UpdateEntity`). Multiple validation
errors are not combined; the user sees the first 4xx that fires. Future
refinement could collect both but is out of scope.

**Layer 4 — Response shape** (`internal/dataentry/api_v1.go`):

Existing `entityToV1` response struct gains a `Warnings []Warning` field,
omitempty. `Warning struct { Code, Path, Detail string }`. Frontend types added
in TKT-B9SXH/TKT-18JS6 when they wire up.

**Layer 5 — SSE event semantics** (RR-RGVJE):
- Entity event fires only if entity properties or content actually changed (existing behavior at api_v1.go:550 — preserved).
- Per-relation events fire only when an actual write occurs (after value-based no-op suppression from Layer 2).
- On 400/422 from any phase, no SSE events fire (handler returns before broadcast).
- A relations-only PATCH that produces no writes (all no-op) emits zero SSE events.

**Layer 6 — Automation/diff staleness** (RR-TK2QF): With the new ordering
(validate → entity update → relation writes), automations fire during entity
update and may create new relations *before* the relation-write phase. Existing
behavior; the relation-write phase doesn't re-validate against the
post-automation graph. We accept this — same hazard as today's legacy
reconciler. Document as a known interaction in api-reference.md.

**Layer 7 — OpenAPI** (`internal/openapi/`):

Update the schema for `PATCH /{plural}/{id}` request body to describe the new
shape (`oneOf` legacy + modern). Add the `warnings` array to all 200 responses.
Add 400 and 422 to documented response set. Regenerate `openapi.yaml`.

**Files to modify:**

- `internal/entitymanager/entitymanager.go` — extend `RelationOptions` (`MetaUnset []string`, `Content *string`); update godoc per RR-4G6V0
- `internal/entitymanager/entitymanagertest/panic.go` — update stub signatures
- `internal/workspace/manager.go` — propagate new fields through to `workspace.updateRelation`
- `internal/workspace/workspace.go` — apply `MetaUnset` after merge in `updateRelation`; honor `*string` content; update godoc
- `internal/mcp/tools_relation.go` — update `Content` argument handling + tool docs
- `internal/cli/link.go` — update `Content: ""` → `Content: nil`
- `internal/lua/runtime.go` — update relation update to use `*string Content`; document Lua API change
- `internal/dataentry/api_v1.go` — handler dispatch + `V1RelationsField`/`V1RelationsUpdate`/`V1ResourceIdentifier` + `Warning` + custom `UnmarshalJSON` + `jsonPointerEscape` helper + handler reorder (validate → entity → relations)
- `internal/dataentry/api_v1_test.go` — add modern-shape tests; keep legacy tests passing
- `internal/dataentry/relations.go` — add `reconcileOutgoingRelationsModern` returning `(warnings, err)`; add same warnings-surface to the legacy reconciler so AC15 produces equivalent results
- `internal/openapi/schemas.go` and/or `paths.go` — schema for new wire shape and warnings response field
- `internal/openapi/openapi.yaml` (derived) — regenerate
- `docs/data-entry/api-reference.md` (new file) — wire-format reference, validation policy summary linking DEC-HWZHA, the data-loss footgun callout, atomicity caveats, automation/diff staleness note, MCP/Lua content-empty-string note
- `CLAUDE.md` — note the new endpoint shape under the data-entry section AND add a Validation policy section linking DEC-HWZHA

**Alternatives considered:**

- **Hard 422 for all soft conditions (the original drift)**: rejected. DEC-HWZHA explicitly governs this. JSON:API wire-shape adoption brought a write-rejection mental model that doesn't fit rela's permissive-storage philosophy.
- **A new method `EntityManager.UpdateEntityWithRelations(ctx, e, rels)` that drives a transaction.** Rejected: store has no transaction primitive (FEAT-CO4YP).
- **Replace the legacy `map[string][]string` shape outright.** Rejected: breaks any external caller; deprecate separately.
- **Keep `workspace.updateRelation` merge-only and fall back to delete+create for meta_unset.** Rejected: doubles SSE event count for a common operation.
- **Use a bool `ClearContent` field instead of `Content *string`.** Rejected: pointer-string is the idiomatic Go way and the call-site cascade is mechanical.
- **Speculatively adding TypeScript types in `frontend/src/api/entities.ts`.** Rejected: nothing consumes them in this PR.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

- **Request body JSON** (HTTP): all fields validated. `relations` map keys: those bound to known relation types proceed; unknown ones return 422 (structural, no storage). Per-edge fields: `type` and `id` required (400 if missing), validated against the graph (target existence, type match) — failures here are warnings, not 422 (DEC-HWZHA). `meta` keys checked against closed schema → unknown keys are warnings. `meta_unset` keys same. `content` only accepted if the relation type has `content: true` (structural 422 otherwise).
- **Request body shape**: legacy + modern shapes are mutually exclusive in one request. Mixing returns 400 with stable code `shape_mixed`, deterministic detail. Custom UnmarshalJSON rejects unknown sibling keys in `V1RelationsUpdate`, non-string `meta_unset` elements, non-array `data`, JSON `null` for the wrapper or for `data`.
- **JSON pointers in error messages**: all path components RFC 6901-escaped. Helper `jsonPointerEscape` applied uniformly.

**Security-Sensitive Operations:**

- **File writes via store** — gated by allowlist validation above. The store backend handles path safety on its own.
- **No new auth surface, no new file-system surface, no crypto.**
- **Warnings expose entity types and IDs** in detail messages — these are already returned in normal API responses, so no new info disclosure.

**Error handling:**

- Internal errors wrap `*relationError` with `Op` / `Reason` / `Target` codes — stable for clients, never include request body bytes.
- Validation errors include the JSON pointer to the offending field but never echo back the metamodel definition or other entities' state.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** see AC1–AC20 above. Each AC maps to a `TestV1Patch_*` test
in `internal/dataentry/api_v1_test.go` using the existing test app harness
(newTestAppV1).

**Edge Cases:**

- **Empty meta map vs absent meta field** — both treated as "no meta change". Testable.
- **Empty meta_unset array vs absent meta_unset field** — both no-op.
- **`content: ""` on a content-bearing relation type** — clears the body. Testable (round-trip).
- **`content: null`** in JSON — pointer is nil after unmarshal; treat as absent.
- **Unicode in IDs and meta values** — pass through as UTF-8.
- **Relation type containing slash or `~`** — JSON pointer must be RFC 6901-escaped.
- **Concurrent PATCH** on the same entity — handler holds `writeMu`; serialized.
- **PATCH that triggers automation that creates more entities** — automation runs during entity update; relation reconcile runs after. Diff is computed against pre-automation graph; that's documented as known interaction.
- **PATCH with empty relations map `{}`** — no-op for relations.
- **Two PATCHes that race each other (one fast-path 404, one stale ETag)** — ETag check inside handler under `writeMu`. Existing behavior.
- **Re-PATCH with same meta value as before**: value-based suppression catches it (RR-M3LWM resolved).
- **`re-PATCHing meta_unset: ["X"]` for absent X**: no-op via value-based suppression (RR-D895N resolved by RR-M3LWM).

**Negative Tests** (each maps to an AC in the 400/422 buckets):

- AC8, AC18, AC19, AC20a, AC20b, AC20c — all 400.
- AC9, AC13 — 422 with stable codes.
- AC17 (validation failure leaves entity AND relations untouched) — tested by snapshotting state before, asserting equal after a relation-422 PATCH.
- Mixed-shape detection robustness — test with two relation types where the legacy one is iterated first AND where the modern one is iterated first; assert error code is `shape_mixed` in both cases (RR-FOLOX). Detail is generic.
- Sibling-key rejection (AC19) — tested with `{"tagged": {"datas":[]}}`.
- Malformed JSON — already covered.

**Warnings tests** (each maps to AC10–AC12c):

- For each soft condition, assert HTTP 200, edge persisted in the expected end state, AND warning present in response with correct `code`, `path`, and a non-empty `detail`. Verify warning `code` matches the corresponding `analyze_*` finding code.

**Integration test approach:** all new tests use the existing in-memory
workspace + store (memstore) via newTestAppV1 — the same harness that exercises
the legacy PATCH path. Tests assert over store state directly (via
`app.outgoingRelations`) AND over the HTTP response (status, body, ETag,
warnings). The no-op suppression test (AC14) subscribes to store events and
asserts zero events fired during the no-op PATCH.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

1. **Risk: changing `RelationOptions.Content` from `string` to `*string` breaks existing callers.**
   - **Mitigation**: type-system-enforced — `go build` fails until every call site is updated. Audit done up front (table in Approach Layer 0); 9 sites + 1 stub. MCP and Lua have subtle semantic notes documented in their tool/API references.
2. **Risk: changing `workspace.updateRelation` from merge-only to merge-then-unset breaks code that relied on un-set keys leaving existing values alone.**
   - **Mitigation**: behavior is purely additive — `MetaUnset: nil` keeps merge-only behavior. Updated godoc makes the contract explicit (RR-4G6V0).
3. **Risk: legacy + modern shape collision detection in custom UnmarshalJSON is buggy and accepts mixed bodies.**
   - **Mitigation**: explicit per-key shape detection with `sawLegacy`/`sawModern` flags. Mixed bodies fail-fast with a stable code, generic detail. Unit tests for the unmarshal in isolation, including both map-iteration orderings (RR-FOLOX).
4. **Risk: atomicity gap — entity update succeeds, then a relation reconcile *write phase* fails partway.**
   - **Mitigation**: validation-first ordering moves validation failures to before entity update (RR-CW1FK). Only mid-write-loop store errors leave a partial state — rare, irrecoverable, and documented in api-reference.md as a known limitation. The legacy reconciler had the same property today; we're not making it worse.
5. **Risk: store backend handles `RelationData.Properties` as full-replacement but `workspace.updateRelation` does merge-then-unset before passing it down. A future store-level call that bypasses the workspace layer would skip the merge.**
   - **Mitigation**: the merge logic is intentionally at the workspace/manager boundary. Document the merge-vs-replace distinction in workspace.go godoc (RR-4G6V0).
6. **Risk: value-based no-op suppression mis-classifies same-meta as a write because of `int` vs `float64` JSON-vs-disk mismatch.**
   - **Mitigation**: in our case both sides come from the same Go-native YAML unmarshal path, so `reflect.DeepEqual` works. Documented assumption. If it breaks in production, fall back to `valueEqual` helper in `internal/model` (small, isolated change).
7. **Risk: shape-based no-op suppression for absent fields (AC14b) is too coarse.**
   - **Mitigation**: superseded by value-based AC14. AC14b is a strict subset and is documented as the trivial fast path.
8. **Risk: warnings response field conflicts with future success/error envelope changes.**
   - **Mitigation**: `warnings` is omitempty and semver-additive. Future envelope work (if any) preserves the field.

**Effort: s** (matches the ticket's existing estimate; the wire-format work is
mechanical, the API extension to `RelationOptions` is small but with
cross-package callers, the warnings surface is straightforward).

## Documentation Planning

For enhancements: identify what documentation needs updating.

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**

- [x] User guide / reference docs — `docs/data-entry/api-reference.md` (new file): wire format, validation policy, atomicity caveats, footgun callout, MCP/Lua content semantics
- [x] CLI help text — N/A
- [x] CLAUDE.md — add Validation policy section linking DEC-HWZHA AND a "Unified PATCH endpoint" subsection in data-entry
- [x] README.md — N/A
- [x] API docs — `internal/openapi/openapi.yaml` regenerated
- [ ] N/A — Internal change, no user-facing docs needed

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:**

- **RR-0NTMQ** (critical, addressed): UnmarshalJSON state machine spelled out in Approach Layer 1.
- **RR-CW1FK** (critical, addressed): validation-first ordering in Approach Layer 3 makes AC17 achievable.
- **RR-M3LWM** (critical, addressed): value-based no-op suppression in Approach Layer 2; AC14 added.
- **RR-QOIXK** (significant, addressed): full call-site enumeration in Approach Layer 0; MCP and Lua semantic notes called out.
- **RR-PZY1W** (significant, addressed): validation order specified in Approach Layer 3.
- **RR-YNWR7** (significant, addressed): RFC 6901 escaping with `jsonPointerEscape` helper.
- **RR-FOLOX** (significant, addressed): stable error code `shape_mixed`, generic detail, both-iteration-orders test.
- **RR-LGK1X** (significant, addressed): null/non-string-element handling specified in Layer 1 sub-state-machine; AC20a added.
- **RR-D895N** (wont-fix under DEC-HWZHA): `meta_unset` of absent key is a no-op; subsumed by value-based AC14.
- **RR-48WQ3** (wont-fix under DEC-HWZHA): legacy and modern both warn (not 422) on missing required meta.
- **RR-RGVJE** (minor, addressed): SSE semantics specified in Approach Layer 5.
- **RR-4G6V0** (minor, addressed): godoc rewrites called out in Approach Layer 0.
- **RR-X5WVA** (minor, addressed): call-site table in Approach Layer 0.
- **RR-UZ8LX** (minor, addressed): `data: null` returns 400 `data_required`; `null` as relation value returns 400 `relation_value_null`.
- **RR-TK2QF** (minor, addressed): automation/diff staleness called out in Approach Layer 6 and api-reference.md.
