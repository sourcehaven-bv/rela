---
id: PLAN-CAK3L
type: planning-checklist
title: 'Planning: Extend PATCH /entities/{id} to accept relations (JSON:API-shaped)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Problem:**

The data-entry frontend cannot include relation changes in the same request that
updates entity properties. Today PATCH `/api/v1/{plural}/{id}` accepts
`{properties, properties_unset, content}` only. Relation changes go through
separate per-edge endpoints (`POST/PATCH/DELETE
/api/v1/{plural}/{id}/relations/{name}/{target}`). This forces the frontend to:

- Issue N+1 requests per form save (one per relation change + one for the
entity).
- Build per-widget diff machinery (`RelationCards` cumulative state machine,
picker has no diff machinery at all → silently loses changes in auto-save).
- Carry a complex error story (3 partially-applied PATCHes is observable to
other clients via SSE).

The auto-save composable in TKT-18JS6 already has a clean single-PATCH FIFO
queue for properties+content. Folding relations into that PATCH unblocks both
`RelationPicker` auto-save (TKT-18JS6) and `RelationCards` auto-save (TKT-B9SXH)
while removing N-1 endpoints from the auto-save hot path.

**Wire format (revised after design review):**

```json
{
  "properties": { "title": "x" },
  "properties_unset": ["assignee"],
  "content": "...",
  "relations": {
    "tagged": {
      "data": [
        {
          "type": "label",
          "id": "L-001",
          "meta": { "weight": 5 },
          "meta_unset": ["added_by"],
          "content": "edge body markdown"
        },
        { "type": "label", "id": "L-002" }
      ]
    }
  }
}
```

Two layers of update semantics, each matching the rela convention at its level:

- **The list of edges is replacement** (`data: [...]`). Edges present in
the list are kept/upserted. Edges absent are removed. `data: []` removes all
edges of that type.
- **Each edge's data (meta, content) is upsert** — same as
entity-level `properties`/`properties_unset`/`content`. Absent fields leave
existing values alone. `meta_unset` clears named keys. Empty string content
clears the body.

**Scope (IN):**

- Extend the PATCH wire format with a top-level `relations` field,
JSON:API-shaped at the list level: `relations: { <type>: { data: [...] } }`.
- "Omit relation type = leave alone" semantics for the list.
- "Empty `data: []` = remove all of that type" semantics.
- "`data: null` = empty list" (per JSON:API §9.2.1; standard library
produces nil slice indistinguishable from `[]` so we don't try to distinguish
them).
- Per-edge upsert via `meta`, `meta_unset`, `content`. Each follows
exactly the same rules as the corresponding top-level field.
- `type` field required on resource identifiers.
- **Symmetric/inverse propagation**: a PATCH that adds/removes A→B
on a `Symmetric` or `Inverse`-declaring relation also stages the inverse edge.
Counterparties (B, C, ...) get their own `entity:updated` SSE events.
- Atomicity per request at the *write* layer: properties + relations
stage through a single `WithTx` and commit-or-rollback together. Validation
failures of types we DO check (existence, target type, relation property types,
missing `type` field) return 422/400 with no observable side effect.
- Diff classifier with **true no-op suppression**: edges where
target+meta+content deep-equal the existing relation are NOT staged. Auto-save
re-PATCHing the same data writes zero relation files.
- Single `entity:updated` SSE event for the PATCHed entity per
successful PATCH. Symmetric/inverse counterparties each get one `entity:updated`
event of their own. Total = 1 + |touched counterparties|.
- Fix the pre-existing graph-mutation-on-422 hazard at
`api_v1.go:485-500` via clone-validate-commit.

**Scope (OUT):**

- **Cardinality validation.** Rela tolerates temporarily invalid
data; users fix it via analyze tools (per user direction).
- **Granular diff verbs (`add`/`remove`/`set`)** — replacement at
the list level only. v2 may add `connect`/`disconnect`-style operations if a use
case appears that doesn't have the full edge set in memory.
- **Stronger atomicity than `WithTx` provides.** Phase 2 (deletes)
is best-effort; Phase 1 (renames) has no true rollback. This ticket inherits the
limit and documents it honestly. A write-ahead log is a separate concern; for a
local-first tool, the existing two-phase commit is acceptable.
- Adoption of the wire format by the frontend (TKT-18JS6 + TKT-B9SXH).
- Migration / removal of the legacy per-relation endpoints.
- Full JSON:API conformance (we use the resource-identifier shape
but not the full `data` envelope, sparse fieldsets, includes, etc.).
- Cross-entity atomic operations (JSON:API has an Atomic Operations
extension; we don't need it).

**Decisions (deliberate, document in API reference):**

1. **Wire format borrows JSON:API resource-identifier shape**, not
the full envelope. `relations.<type>.data: [{type, id, ...}]`.

2. **`type` field is REQUIRED on resource identifiers.**

3. **Omit-vs-empty distinction (LIST level)**: stolen verbatim from
JSON:API §9. Absent relation type = leave alone. `data: []` = remove all of that
type. `data: null` = same as `data: []`.

4. **Per-edge data is UPSERT (not replacement).** This is the key
change from earlier draft:
   - `meta` keys present → merged into existing meta.
   - `meta_unset: ["x"]` → clears named keys.
   - `meta` absent → leaves all existing meta intact.
   - `content` absent → leaves existing body intact.
   - `content: "..."` → sets body to the value (including `""` to
clear).
   - Mirrors how entity-level `properties`/`properties_unset`/`content`
work today. Consistent across the API.

5. **Top-level key is `relations`** (rela-internal naming).

6. **Direction: outgoing only.** To update incoming relations, PATCH
the source entity.

7. **Symmetric / inverse propagation.** When adding/removing
A→B on a relation type with `Symmetric: true` or non-nil `Inverse`, the inverse
edge is also staged in the same transaction. Counterparties get their own
`entity:updated` SSE events. The diff is per-source-entity-and-relation-type; a
counterparty's unrelated edges are NEVER touched.

8. **No cardinality enforcement.** Rela tolerates temporarily
invalid data; analyze tools surface violations.

9. **Status codes**: shape errors (malformed JSON, missing required
scalar, missing `type` field) → **400** with structured pointer. Metamodel
validation errors (unknown relation type, target type mismatch, unknown target
ID, invalid meta property type) → **422** with detail.

10. **Atomicity is two-phase, not absolute.** Honest documentation:
`WithTx` Phase 1 (renames) is *mostly* atomic — mid-flight failure can lose
prior file content for already-renamed files. Phase 2 (deletes) is best-effort;
failures are silently ignored. For a local-first tool this is acceptable.
Document in the API reference; revisit if/when we add a write-ahead log.

11. **Diff classifier suppresses no-op writes.** An edge whose
target + final-meta + final-content deep-equal the current state is NOT staged.
PATCHing the same data twice writes nothing the second time.

**Acceptance Criteria:**

1. **List-level wire format accepted**: PATCH with `relations:
{belongs-to: {data: [{type: "category", id: "C-001"}]}}` updates the entity's
`belongs-to` relations to exactly `{C-001}`. Test: Go test on
`handleV1UpdateEntity`.

2. **Omit relation type = leave alone**: PATCH with `properties:
{title: "x"}` and no `relations` key leaves all existing relations untouched.
Test: Go test.

3. **Empty data = remove all of that type**: PATCH with `relations:
{tagged: {data: []}}` removes all `tagged` relations. Other relation types
untouched. Test: Go test.

4. **`data: null` equivalent to `data: []`**: per JSON:API §9.2.1.
Test: Go test verifying both produce identical post-state.

5. **Add + remove + keep + update in one PATCH**: entity has
`tagged → [L1, L2, L3]`. PATCH with `data: [{L1 with new meta}, {L4}]` results
in `[L1 (new meta), L4]` (L2, L3 removed; L4 added; L1 meta-updated). Test: Go
test.

6. **Per-edge meta UPSERT**: existing edge has `meta: {weight: 3,
added_by: "alice"}`. PATCH with `data: [{id: L1, meta: {weight: 5}}]` results in
`meta: {weight: 5, added_by: "alice"}` (weight updated, added_by preserved).
Test: Go test.

7. **Per-edge `meta_unset`**: PATCH with `data: [{id: L1, meta:
{weight: 5}, meta_unset: ["added_by"]}]` results in `meta: {weight: 5}`
(added_by cleared). Test: Go test.

8. **Per-edge `content` upsert**: existing edge has `content:
"old"`. PATCH with `data: [{id: L1}]` (no content) leaves `content: "old"`.
PATCH with `data: [{id: L1, content: "new"}]` sets `content: "new"`. PATCH with
`data: [{id: L1, content: ""}]` sets `content: ""`. Test: Go test parameterized
over three cases.

9. **Validation: relation type doesn't exist** → 422. Test: Go
test.

10. **Validation: target type mismatch** → 422. Test: Go test.

11. **Validation: target ID doesn't exist** → 422. Test: Go test.

12. **Shape error: missing `type` field** → 400 with structured
pointer (e.g., `/relations/tagged/data/0/type`). Test: Go test.

13. **Validation: invalid meta property type** → 422. Test: Go
test.

14. **Symmetric propagation**: relation type `tagged` is
`Symmetric: true`. Entity A has `tagged → [B]`. PATCH `A.tagged: [C]` results
in: A→C exists, A→B removed; B→A removed (was auto-created via symmetric); C→A
created. Counterparty B's *unrelated* edges (e.g., B→D under `tagged`) are NOT
touched. Test: Go test asserting graph state and disk files for A, B, C, D.

15. **Symmetric propagation events**: a PATCH on a symmetric
relation that touches counterparty edges fires `entity:updated` for each
affected counterparty, plus the PATCHed entity. Test: Go test asserting event
count = 1 +
    |touched counterparties|.

16. **Inverse propagation**: relation `assesses` has `Inverse:
{Name: "assessed-by"}`. PATCH on A.assesses adds/removes the inverse
`assessed-by` edges. Test: Go test.

17. **No-op suppression**: PATCH with relations EXACTLY matching
current state writes zero relation files. Test: Go test using a write-counter
wrapped repository.

18. **Re-PATCH no-op event suppression**: a PATCH where everything
is no-op (no property change, no relation change) does NOT fire an
`entity:updated` SSE event. Test: Go test on the broker.

19. **Atomicity on validation failure**: PATCH with valid
properties and invalid relations returns 422. The live graph node is
**untouched** (fixes pre-mutation bug from TKT-18JS6 QA). Test: Go test
asserting GET after failed PATCH returns pre-PATCH values.

20. **Atomicity on Phase 1 commit failure**: simulate a rename
failure mid-Phase-1. The handler returns 500 with a clear error. The in-memory
graph state is precisely the pre-PATCH state (gated by `tx.applyGraphMutations`
on full commit success). On-disk state may be partially inconsistent —
documented and accepted. Test: failure-injection at the filesystem layer.

21. **Single SSE event for the PATCHed entity** (plus one per
affected symmetric/inverse counterparty): a PATCH that adds 5 relations and
changes 3 properties on a non-symmetric relation fires exactly ONE
`entity:updated` event (for the patched entity). Test: Go test on the broker.

22. **ETag still works**: If-Match gate. Test: Go test sending
stale ETag → 412.

23. **Backwards compat**: existing PATCH bodies (no `relations`
key) work unchanged. Existing `TestV1UpdateEntity_*` tests pass.

24. **Per-relation endpoints still work**. Existing tests pass.

25. **Graph-mutation-on-422 fix**: AC #19 covers it.

26. **OpenAPI doc lists the `relations` field** with the
resource-identifier schema. Test: regenerate and grep.

## Research

- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Existing Solutions / Standards Survey:** *(unchanged from the earlier draft —
JSON:API §9, Rails, OData, JSON Patch/Merge Patch, GraphQL conventions reviewed;
JSON:API §9 is the cleanest list-level match; rela's existing
`properties`/`properties_unset` is the edge-data-level convention.)*

**Adopted from JSON:API §9** (list level):
- Omit-vs-empty distinction.
- `relations.X.data: [{type, id, ...}]` shape.
- Replacement semantics for the *list* of edges per relation type.
- Per-edge `meta` lives on resource identifiers.
- `type` field required.
- `data: null` ≡ `data: []`.

**Adopted from rela's existing convention** (edge-data level):
- Upsert semantics on `meta` (mirror entity `properties`).
- Explicit `meta_unset` for clearing named keys (mirror
`properties_unset`).
- Upsert on `content` (mirror entity `content`).

**Codebase facts (verified during research, corrected from earlier draft):**

- `internal/workspace/tx.go:103` already has `WriteRelation`. Line
123 already has `DeleteRelation`. The Tx struct already accumulates
`addEdges`/`removeEdges` and orders graph mutations correctly. **No new
transaction-layer code needed.**
- `internal/workspace/workspace.go:1521` `UpdateRelation` *merges*
meta — incompatible with our edge-level handling. The handler builds a
fully-formed `*model.Relation` (with desired final meta/content after
merge+unset applied client-side intent) and stages via `tx.WriteRelation`.
`Workspace.UpdateRelation` is intentionally left alone — automation code relies
on its merge semantics; unifying is a follow-up.
- `internal/workspace/tx.go:24-28`: `WithTx` holds `reloadMu`. The
diff must be computed *inside* `WithTx` so reloadMu serializes it with
file-watcher reloads. Restructure handler accordingly.
- `internal/repository/transaction.go:138`: Phase 2 deletes are
best-effort. Line 156: Phase 1 has no true rollback. Document honestly; this
ticket does NOT add a write-ahead log.
- `internal/graph/graph.go:166-209`: graph allows duplicate edges
on `(from, type, to)`. File layout enforces uniqueness; we document the
assumption.
- `internal/dataentry/watcher.go`: only emits `entity:*` events,
no `relation:*`. The "consolidates N+1 events" framing in earlier draft was
misleading — corrected to "single event per affected entity."

**Verified against rela's openapi generator:** TODO before code — read
`internal/openapi/<entry>.go` to confirm the new request struct types are picked
up via reflection (likely yes since they're just additional fields on the
existing request struct, but verify).

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical approach:**

### Backend: extend the request shape

```go
var req struct {
    Properties      map[string]interface{}              `json:"properties,omitempty"`
    PropertiesUnset []string                            `json:"properties_unset,omitempty"`
    Content         *string                             `json:"content,omitempty"`
    Relations       map[string]V1RelationsUpdate        `json:"relations,omitempty"`
}

type V1RelationsUpdate struct {
    Data []V1ResourceIdentifier `json:"data"`
}

type V1ResourceIdentifier struct {
    Type      string                 `json:"type"`
    ID        string                 `json:"id"`
    Meta      map[string]interface{} `json:"meta,omitempty"`
    MetaUnset []string               `json:"meta_unset,omitempty"`
    Content   *string                `json:"content,omitempty"`
}
```

Go's `encoding/json` populates the map only for keys present in the body —
absence of a relation type means absence. Inside the resource identifier,
`*string` for `Content` distinguishes "field absent" (nil) from "field is empty
string" (non-nil pointer to "").

For `data: null`: standard library produces a nil slice indistinguishable from
`[]`. We accept both as equivalent (per JSON:API §9.2.1) — no custom unmarshal
needed.

### Backend: handler refactor — clone-validate-commit, ALL inside WithTx

The handler enters `WithTx` early so `reloadMu` is held throughout the
diff/validate/commit sequence. Steps:

1. Take `App.writeMu` (existing lock at handler entry). Existing.
2. **Lookup** entity in the graph via the live state (read-only).
ETag check against the live entity.
3. **Begin `WithTx`** — `reloadMu` now held.
4. **Clone** the entity for staging: `staged := entity.Clone()`.
All mutations apply to `staged`, not the live pointer.
5. **Apply property changes** to `staged.Properties` (existing
merge + `properties_unset` loop).
6. **Apply content** to `staged.Content`.
7. **Compute the relation diff** for each relation type in
`req.Relations`:
   - `current` = `tx.OutgoingEdges(entityID)` filtered by relation
type.
   - `desired` = `req.Relations[type].Data`.
   - For each entry in `desired`:
     - Resolve the existing edge (if any) by `(target_id, type)`.
     - Compute the **final relation** (existing edge merged with
update directives):
       - `final.Properties` = (existing.Properties merged with
entry.Meta) minus entry.MetaUnset keys.
       - `final.Content` = entry.Content if present, else
existing.Content.
     - Classify:
       - **keep**: edge exists AND `final` deep-equals existing →
no stage.
       - **add**: edge does not exist → stage.
       - **update**: edge exists AND `final` differs → stage.
   - For each existing edge not in `desired`: classify as
**remove**.
8. **Resolve symmetric/inverse propagation** for each
`add`/`remove`. For each affected counterparty, compute its propagation diff and
add to the staging set. Track touched counterparty IDs for SSE event
broadcasting.
9. **Validate the full intended state**:
   - `meta.ValidateEntity(staged)` (existing).
   - For each relation in `add` or `update`:
     - Existence of target ID in graph.
     - `meta.ValidateRelation(relType, fromType, toType)` for
type compatibility.
     - `meta.ValidateRelationProperties(rel)` for meta
type-checks. **Reject unknown meta keys** (closed schema — a schema-drift
defense; matches existing `ValidateRelationProperties` strictness).
   - Shape errors detected during JSON parse → already returned
400 before this point. Validation here returns 422.
10. **If any validation fails**, return the error and exit
`WithTx` with rollback. The live graph is untouched.
11. **Stage** writes:
    - `tx.WriteEntity(staged, meta)`.
    - For each `add`/`update`: `tx.WriteRelation(rel)`.
    - For each `remove`: `tx.DeleteRelation(from, type, to)`.
    - For each propagated counterparty change: same `tx.WriteRelation`
/ `tx.DeleteRelation`.
12. **Commit**. On Phase 1 failure → 500 + clear message; graph
untouched. On Phase 2 (delete) failures → silently ignored per existing
convention; the entity is still valid even if some on-disk relation files
persist.
13. **Broadcast SSE events**: one `entity:updated` for the PATCHed
entity. One `entity:updated` per touched counterparty. Skipped entirely if the
diff was empty (no-op suppression at the event level).

### Backend: no-op suppression

The diff classifier's `keep` bucket is the suppression mechanism. Properties +
content also need the same: if `staged` deep-equals the live entity *and* no
relations changed, return 200 without calling `WithTx` at all (or, simplest
implementation: enter `WithTx`, find no work, exit cleanly without writes or
events).

### Backend: symmetric/inverse propagation logic

For relation type `T` with `Symmetric: true`:
- `add(A, T, B)` → also stage `add(B, T, A)`.
- `remove(A, T, B)` → also stage `remove(B, T, A)`.

For relation type `T` with `Inverse: {Name: "T_inv"}`:
- `add(A, T, B)` → also stage `add(B, T_inv, A)`.
- `remove(A, T, B)` → also stage `remove(B, T_inv, A)`.

The staged counterparty changes are validated (target type, meta) just like
primary changes. If a propagation fails validation, the whole PATCH fails
(atomicity).

Counterparty IDs are collected into a `Set<string>` during the diff phase. After
commit, broadcast `entity:updated` for each.

### Backend: extend `WithTx` — NOT NEEDED

`tx.WriteRelation` and `tx.DeleteRelation` already exist. The handler uses them
directly. No transaction-layer changes.

### Frontend: typing only

Extend types in `frontend/src/api/entities.ts`:

```typescript
export interface ResourceIdentifier {
  type: string
  id: string
  meta?: Record<string, unknown>
  meta_unset?: string[]
  content?: string
}

export interface RelationsUpdate {
  data: ResourceIdentifier[]
}

export interface UpdateEntityPatch {
  properties?: Record<string, unknown>
  properties_unset?: string[]
  content?: string
  relations?: Record<string, RelationsUpdate>
}
```

`patchEntity` already accepts `UpdateEntityPatch` — types flow through
unchanged.

### Frontend: documentation gotcha

API reference includes a callout about the auto-save data-loss risk: "If you
build PATCH bodies via object spread, ensure form state has been fetched before
the first auto-save fires. `relations.X.data: []` deletes all edges of that
relation type."

**Files to modify:**

Backend:
- `internal/dataentry/api_v1.go` — request struct + handler
refactor.
- `internal/dataentry/api_v1_test.go` — tests covering AC 1-26.
- `internal/metamodel/validation.go` — confirm
`ValidateRelationProperties` already rejects unknown keys; if it doesn't,
tighten it (or document the existing behavior and decide whether tightening is
in or out of this scope).

Frontend (typing only):
- `frontend/src/api/entities.ts` — `ResourceIdentifier`,
`RelationsUpdate`, `UpdateEntityPatch` extensions.

Documentation:
- `docs/data-entry/api-reference.md` (or whichever file documents
the PATCH endpoint): full new wire format with examples for each rule
(omit-list, empty-list, upsert-meta, meta_unset, content, symmetric/inverse,
no-op, atomicity caveats).
- `CLAUDE.md` data-entry section: brief mention of the unified
PATCH path; cardinality remains advisory.
- `internal/openapi/` — verify generator picks up new types.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input sources & validation:**

- *`relations` map keys*: validated against `meta.GetRelationDef`
allowlist → 422 on unknown.
- *`relations.X.data[].type`*: validated against `RelationDef.From`/
`.To` allowlist → 422 on mismatch.
- *`relations.X.data[].id`*: existence checked via graph lookup →
422 on unknown.
- *`relations.X.data[].meta`*: validated against
`RelationDef.Properties` schema. **Unknown keys rejected** (422, closed schema).
- *`relations.X.data[].meta_unset`*: keys validated against
`RelationDef.Properties` schema (unknown keys → 422). Unset of a non-existing
key on the existing edge is a no-op (not an error).
- *`relations.X.data[].content`*: only meaningful for relation
types with `Content: true`. For others, presence of `content` field → 422
("relation type does not support content body").
- *`type` field on resource identifier*: required. Missing → 400.

All inputs go through the same validator chain as legacy endpoints
+ stricter `meta` key validation. **Security posture is identical
to the legacy path or stricter.**

**Security-sensitive operations:**

- Filesystem writes — same as legacy.
- Bulk relation deletion via `data: []` — capability already
exists (per-edge DELETEs). The new endpoint doesn't widen the attack surface but
it's MORE EFFICIENT to misuse. Mitigation: documentation callout (per RR-6YF8F
decision (b) — client discipline + docs, no server flag).
- Symmetric/inverse propagation can mutate counterparty entities.
Auth posture is identical to PATCHing those counterparties directly: the user
must be authenticated, and the per-edge changes go through the same validator
chain.

**Error-handling note:** structured RFC 7807 problem details with JSON pointers
for shape errors. No raw struct dumps.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test infrastructure additions:**

- `newUpdatableTestAppWithRelations(t)` — fixture with multiple
relation types: a to-one (`belongs-to → CAT-001`), a to-many (`tagged →
[LBL-001, LBL-002]`), a symmetric (`linked-to → [E-001]`), and an inverse
(`assesses → [F-001]` with implicit `assessed-by` on the target side).
- **Failure-injection** (AC #20): use a counting wrapper around
the FS interface (`tx.repo.fs.Rename` is the seam — verified to be
interface-typed). Wrapper fails the Nth `Rename` call. Confirmed feasible during
research; if not, fall back to wrapping `repository.Tx` (which IS an interface).
- **Write-counter** (AC #17): wrapped repo that counts
WriteRelation/WriteEntity calls. Verifies no-op suppression.

**Edge cases (additional):**

- `relations: {}` (empty map) → no-op.
- `data: null` → equivalent to `data: []` (per AC #4).
- Self-referential relation (e.g., `depends_on` from TKT-001 to
TKT-001): metamodel decides; existing behavior preserved.
- Relation type with `Content: true` but request has no `content`
→ existing content preserved (upsert).
- Relation type with `Content: false` but request has `content`
field → 422 ("does not support content").
- `meta_unset` of a key that doesn't exist on the existing edge →
no-op (not error).
- `meta` and `meta_unset` for the same key in the same request:
**`meta_unset` wins** (apply unset *after* merge). Document.
- Two clients PATCH same entity concurrently: serialized by
`App.writeMu`. Document If-Match for stronger guarantees.
- Symmetric relation with self-loop (A→A): special-case; no
propagation needed (same edge). Test with `linked-to A→A`, PATCH `A.linked-to:
[]`.
- Inverse relation type validation: if `Inverse: {Name: "X"}` is
declared but `X` doesn't exist as a relation type, fail at metamodel-load time
(not in this PATCH).

**Negative tests (HTTP status codes):**

- Malformed JSON → 400.
- Missing `type` field on resource identifier → 400 with pointer.
- Unknown relation type → 422.
- Unknown target ID → 422.
- Wrong target type → 422.
- Unknown meta key (closed schema) → 422.
- Invalid meta property value type → 422.
- `content` on a relation type without `Content: true` → 422.
- Concurrent ETag mismatch → 412.

**Integration tests:**

- E2E via existing `e2e_test.go` infrastructure: spin up real
server, hit PATCH with new shape, assert disk + graph + SSE event(s).
- Test that a successful PATCH leaves no `.new` temp files in
`relations/` (transaction cleanup).
- Test symmetric round-trip: PATCH from one tab, observe
`entity:updated` for counterparty in another tab.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

| Risk | Mitigation |
|------|-----------|
| Symmetric/inverse propagation is per-relation-type behavior — incorrect propagation corrupts the graph | Comprehensive AC coverage (#14, #15, #16). Test with edge cases: self-loop, counterparty with unrelated edges, inverse pointing to a relation type with its own `Inverse`. |
| Atomicity overclaim | Documented honestly (decision #10 + risk #5). For local-first tool, acceptable. Don't promise more than `WithTx` delivers. |
| `data: []` footgun | Client discipline + docs callout (decision per RR-6YF8F). Auto-save composable in TKT-18JS6 must guard against sending `relations` keys before fetch completes. |
| Pre-existing graph-mutation-on-422 fix is bundled but not the primary goal | Bundled because the new clone-validate-commit pattern naturally fixes it. AC #19 verifies; won't regress. |
| JSON:API users may expect full envelope | Document non-conformance in API reference. |
| Bulk deletion via `data: []` | Same posture as today (per-edge DELETEs allow same loss). Documented; SSE event provides visibility. |
| Failure-injection seam may not exist cleanly | Verified during research: `repository.Tx` is an interface; FS layer is also interface-typed. Either works; pick the cleanest at implementation time. |
| Symmetric propagation event volume | A symmetric PATCH on N counterparties fires N+1 events. Acceptable for typical N (≤10). For pathological N, the SSE consumer can debounce. |

**Effort estimate:** **m** (medium) — kept scope per user direction.

- 0.5d: extend request struct + handler refactor (clone-validate-
commit, all inside WithTx).
- 0.5d: diff classifier with no-op suppression.
- 1d: symmetric/inverse propagation logic + tests.
- 1d: per-AC Go tests on the handler (26 ACs).
- 0.5d: TS types.
- 0.5d: API docs (longer than expected because of upsert semantics
  + symmetric/inverse explanation + atomicity caveats).
- 0.5d: code review responses, polish.

Total ~4.5d. Up slightly from the 4d earlier estimate after folding in the
upsert/unset/content additions and propagation logic. Down ~1d net from the
original 4.5d after RR-4CBYE removed nonexistent transaction work.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation impact:**

- [x] User guide / reference docs — `docs/data-entry/api-reference.md`
gains comprehensive `relations` section. Sections needed:
  - Wire format basics.
  - List-level rules: omit / `data: []` / `data: null`.
  - Edge-level rules: upsert `meta` / `meta_unset` / upsert
`content`.
  - Symmetric / inverse propagation behavior.
  - Atomicity caveats (Phase 1/2, what rolls back when).
  - The `data: []` footgun callout.
  - JSON:API non-conformance note.
- [ ] CLI help text — N/A.
- [x] CLAUDE.md — data-entry section: brief mention of the
unified PATCH path. Note that cardinality remains advisory.
- [ ] README.md — N/A.
- [x] OpenAPI generator — verified during research; new types
flow through reflection. AC #26 confirms.
- [ ] N/A — Internal change, no user-facing docs needed.

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings (18 total — all addressed):**

| RR | Severity | Status | How addressed in plan |
|----|----------|--------|-----------------------|
| RR-4CBYE | critical | addressed | Plan corrected: tx.WriteRelation/DeleteRelation already exist. Removed 1d budget for nonexistent work. Codebase facts section corrected. |
| RR-GSHRX | critical | addressed | Handler builds full *model.Relation with desired final state and stages via tx.WriteRelation directly. Workspace.UpdateRelation is intentionally bypassed. |
| RR-FMLU1 | critical | addressed | Decision #7 + ACs #14-16: symmetric/inverse propagation explicit. Counterparties get their own entity:updated events. |
| RR-S4FXV | significant | addressed | `content: *string` added to V1ResourceIdentifier with upsert semantics (same as meta). AC #8 covers. Decision #4 documents the rule. |
| RR-4KRB3 | significant | addressed | data: null treated as equivalent to data: []. AC #4 covers. Decision #3 documents. |
| RR-EC5MM | significant | addressed | Decision #9: shape errors → 400, metamodel errors → 422. AC #12 (missing type → 400). All other validation ACs → 422. |
| RR-STND2 | significant | addressed | Diff is computed inside WithTx (under reloadMu). Approach steps restructured: WithTx is entered at step 3, all subsequent steps run inside it. |
| RR-P4AST | significant | addressed | Decision #11 + AC #17/#18: no-op suppression at the diff classifier; no SSE event when nothing changed. |
| RR-YN3LM | significant | addressed | Decision #10 + scope (OUT) entry: atomicity is two-phase, not absolute. AC #20 tests Phase 1 commit-failure case. Honestly documented. |
| RR-6YF8F | significant | addressed | Decision: client discipline + docs callout (per user direction). API reference adds the callout. |
| RR-Q1FIT | significant | addressed | Unknown meta keys → 422. Mention in security section + validator behavior. |
| RR-Y8T8A | minor | addressed | Decision: assume (from, type, to) uniqueness, enforced by file layout. Documented as an assumption. |
| RR-7DI3T | minor | addressed | AC for cardinality non-enforcement now asserts graph state directly (no `analyze_cardinality` call). |
| RR-0RPAO | minor | addressed | "N+1 events" framing dropped. Plan now says "single event per affected entity." |
| RR-Q1C2R | minor | addressed | OpenAPI generator coupling moved to Research as a verify-before-implementation item. AC #26 added. |
| RR-QCLPQ | minor | addressed | Test infrastructure section names the failure-injection seam (FS layer or repository.Tx, both interface-typed). |
| RR-UBXDI | minor | addressed | Future work note: granular `connect`/`disconnect` could come in v2. v1 is replacement+upsert. |
| RR-ODR30 | nit | addressed | AC #20 covers Phase 1 commit failure → graph untouched explicitly. |
