---
id: PLAN-ACI8
type: planning-checklist
title: 'Planning: Reorderable relations via metamodel-declared ordering property'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

IN scope:

- New `orderable: outgoing | incoming | both` enum on a relation type in `metamodel.yaml`. Absent or empty = not orderable.
- Managed float ordering properties on every instance of an orderable relation. The property names are **always** `_order_out` (outgoing side) and `_order_in` (incoming side), regardless of mode. A mode of `outgoing` manages `_order_out` only; `incoming` manages `_order_in` only; `both` manages both.
- Sort the relevant side(s) at the API layer when the type is orderable (stable fallback for ties / missing values).
- Data-entry detail screen: drag-to-reorder of related items in an orderable list section on whichever side(s) are enabled.
- Backend PATCH path that accepts the new order value(s) per relation.
- Analyze warning (not error) for duplicate or missing order values on orderable relations, per enabled side.

OUT of scope (defer to follow-up tickets):

- Sorting by a user-named property (we use the underscore-managed property names).
- Ordering of entity lists outside relation context (already user-driven via list views).
- Lua reorder helpers — the existing relation update path is sufficient.
- Bulk re-rank operations across many parents.
- A separate UI for editing order properties as free-form numbers — the drag interaction is the only edit affordance.
- Polymorphic restriction tickets: relations with multiple `to:` types remain allowed and produce one mixed list, globally ordered.

**Acceptance Criteria:**

1. **AC1 — Schema acceptance.** A relation type with `orderable: outgoing`, `orderable: incoming`, or `orderable: both` in `metamodel.yaml` loads without error and exposes its orderability through the metamodel API. An invalid value (e.g. `orderable: "yes"`) fails the loader at startup. *Test:* unit test on the metamodel loader, table-driven across the three valid values plus an invalid one.
2. **AC2a — Outgoing-side API returns sorted order.** For `outgoing` or `both`, `GET /api/v1/entities/{type}/{id}/relations` returns outgoing edges of the orderable type sorted ascending by `_order_out`. Edges with a missing value sort *last*, ties broken stably by relation file name. *Test:* `api_v1_test.go` table-driven case.
3. **AC2b — Incoming-side API returns sorted order.** For `incoming` or `both`, the incoming-side listing is sorted ascending by `_order_in`, same tie-break. *Test:* `api_v1_test.go` table-driven case.
4. **AC3 — Drag-to-reorder persists.** Dragging an item in an orderable list section issues a PATCH that sets the appropriate order property to the midpoint of the two new neighbors, and only the moved relation is updated. *Test:* integration test asserts exactly one relation file changed plus a Playwright e2e covering both outgoing and incoming when applicable.
5. **AC4 — Tolerant rendering.** A list of relations with missing or duplicate order values still renders without crashing and produces a deterministic order. *Test:* unit test with fixtures `[1.0, 1.0, nil, 2.0]` asserting deterministic stable output.
6. **AC5 — Analyze surfaces issues.** `analyze` reports duplicate or missing order values on orderable relations as warnings (severity `warning`, not `error`), per enabled side. *Test:* `analyze.go` test with mixed valid/invalid fixtures across all three modes.
7. **AC6 — Non-orderable relations unaffected.** A relation type without `orderable:` ignores any `_order_*` property; the API does not sort it. *Test:* regression test on existing relation fixtures.
8. **AC7 — Insert / append.** Adding a brand-new relation auto-assigns the order property on each enabled side: `(max_existing + 1.0)` on the existing-siblings side, `1.0` if no siblings on that side. *Test:* unit tests on the relation-create path for all three modes (outgoing-only, incoming-only, both).
9. **AC8 — Renumber on precision collapse.** When midpoint inserts shrink the gap between two neighbors below `1e-9`, the API renumbers the affected siblings to integer ordinals `1.0, 2.0, …` in a single batch, on whichever side collapsed. *Test:* unit test that forces collapse and asserts a single renumber.
10. **AC9 — `both` mode is independent per side.** Reordering on the outgoing side never touches `_order_in` (and vice versa). *Test:* dedicated regression case.
11. **AC10 — Mode change preserves existing order values.** Changing a relation type from `outgoing` to `both` (or `incoming` to `both`, or vice versa) leaves the existing on-disk order values intact and immediately useful — because the property names are stable. *Test:* metamodel-mode-change test: load a fixture under `outgoing`, switch the metamodel to `both`, verify the same `_order_out` values still drive the outgoing-side sort and `_order_in` is simply missing (treated as "to be assigned on next write or via analyze warning").

## Research

- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Existing Solutions:**

- **Sparse float midpoint (LexoRank-lite).** Atlassian's LexoRank uses string fractional indices for the same problem. For our scale (relations per parent rarely > a few hundred), an IEEE-754 `float64` gives ~52 bits of precision — well over a billion midpoint inserts before precision collapse — and is human-readable in markdown frontmatter. Renumber-on-collapse is the fallback; expected to be rare. Rejected true LexoRank strings: extra dependency, harder to hand-edit. Rejected dense integers: every reorder rewrites N siblings.
- **Existing relation properties.** `FEAT-jsjj` already added typed properties on relations (`entity.Relation.Properties map[string]interface{}`, see `internal/entity/entity.go:207`). Order values are just more typed properties. No new storage plumbing needed.
- **Existing relation update path.** `dataentry/relations_modern.go` plus the V1 wire format (`relations_v1_wire.go`) already supports per-edge meta updates. We extend the existing flow rather than adding a separate "reorder" endpoint.
- **Existing DnD pattern in the frontend.** `frontend/src/views/KanbanView.vue:214–281` already uses **native HTML5 DnD** (`@dragstart`, `@dragover`, `@drop`, `@dragend`). No DnD library in `package.json`. We follow the same pattern for relation reorder — no new dependency.
- **No existing sort on relations.** `default_view.go:70` sorts only relation-type *group names*, not instances; the per-type response is built in iteration order in `api_v1.go:690–734`.
- **Underscore-prefix is not reserved.** `ReservedPropertyNames` (`metamodel/types.go:198–203`) only reserves `id` and `type`. `_order_in` and `_order_out` are safe.

**Reference Implementations:**

- `frontend/src/views/KanbanView.vue` (in-tree) — pattern for native HTML5 DnD with a `draggedItem` ref + handlers. Reuse the shape.
- Notion-style "manage order via fractional index" — backend ordering scheme.
- LexoRank (Atlassian, Jira) — original implementation of the ordering scheme.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

### 1. Metamodel schema

Add a typed enum field `Orderable` to `metamodel.RelationDef`:

```yaml
relations:
  has-step:
    from: [recipe]
    to: [step]
    orderable: outgoing   # outgoing | incoming | both | (absent)
```

Go type:

```go
type OrderableMode string

const (
    OrderableNone     OrderableMode = ""
    OrderableOutgoing OrderableMode = "outgoing"
    OrderableIncoming OrderableMode = "incoming"
    OrderableBoth     OrderableMode = "both"
)

// Reserved property names for relation ordering.
const (
    OrderPropertyOut = "_order_out"
    OrderPropertyIn  = "_order_in"
)

type RelationDef struct {
    // ...existing fields...
    Orderable OrderableMode `yaml:"orderable,omitempty"`
}

// Sides that are orderable for this relation type.
// Returns "" for a side that is not orderable.
func (r RelationDef) OutgoingOrderProperty() string {
    if r.Orderable == OrderableOutgoing || r.Orderable == OrderableBoth {
        return OrderPropertyOut
    }
    return ""
}
func (r RelationDef) IncomingOrderProperty() string {
    if r.Orderable == OrderableIncoming || r.Orderable == OrderableBoth {
        return OrderPropertyIn
    }
    return ""
}
```

**Property naming rule (per user feedback):** the property names are **always**
`_order_out` and `_order_in`, regardless of mode. This makes the on-disk
representation stable across metamodel changes. If a user starts with
`orderable: outgoing` and later promotes to `both`, the existing `_order_out`
values on disk continue to drive outgoing-side order — no migration, no re-sort,
no silent value loss. The added side simply has missing values until the next
write or until a `Renumber` runs.

The helper methods centralize the name mapping so call sites never spell the
strings directly. The constants `OrderPropertyOut` / `OrderPropertyIn` are
referenced everywhere a name is needed.

Polymorphic relations (multiple `to:`) are allowed and produce one mixed
sortable list per enabled side.

### 2. Backend reorder helpers

New package-level helpers in `internal/entitymanager/order.go` (pure functions,
side-agnostic):

```go
// MidpointOrder returns a value strictly between a and b. If the gap is
// below collapseThreshold, returns (0, false) so the caller renumbers.
func MidpointOrder(a, b float64) (float64, bool)

// AppendOrder returns the next ordinal after the given existing values.
func AppendOrder(existing []float64) float64

// PrependOrder returns a value below the smallest existing value.
func PrependOrder(existing []float64) float64

// NeedsRenumber reports whether a sorted list has any adjacent gap below
// the precision threshold.
func NeedsRenumber(sorted []float64) bool

// SortRelations returns the input in stable sort order by the named
// property, with missing/non-numeric values last.
func SortRelations(rels []entity.Relation, prop string) []entity.Relation
```

These are unit-testable in isolation and used by both sides.

### 3. API integration

- `handleV1EntityRelations` in `dataentry/api_v1.go`: after building the per-type slice, look up the type in the metamodel; if its outgoing-order property is set, sort by `OrderPropertyOut`. Mirror logic for the incoming-side listing using `OrderPropertyIn`.
- `V1ResourceIdentifier` in `relations_v1_wire.go`: accept the order property inside the existing `Meta` map. No top-level `order` field. The UI sends the new midpoint via the same PATCH the meta-update flow uses today.
- `entitymanager.CreateRelation`: when the type is orderable, auto-assign on each enabled side:
  - outgoing-enabled → compute `AppendOrder` over the existing outgoing siblings' `_order_out` values of the new source entity.
  - incoming-enabled → compute `AppendOrder` over the existing incoming siblings' `_order_in` values of the new target entity.
- Renumber-on-collapse: the write path detects collapse after the PATCH and triggers a renumber on the affected side in the same operation.

### 4. Frontend integration

DnD approach: **native HTML5 events**, same pattern as `KanbanView.vue`. No new
frontend dependency.

Files:

- `frontend/src/components/forms/RelationCards.vue` — when the relation type has the relevant side enabled, attach `draggable="true"` to each card and handle `@dragstart` / `@dragover` / `@drop` / `@dragend`. Track `draggedRelationId` in a ref local to the component.
- New small composable `frontend/src/composables/useRelationReorder.ts` — receives `(propertyName, neighbors, moved)`, computes the midpoint via a small TS port of `MidpointOrder`, dispatches PATCH with `{ id, meta: { [propertyName]: <new value> } }` via the existing `updateRelations` API client. The property name is supplied by the caller from metamodel metadata — the composable never hard-codes `_order_out` or `_order_in`.
- `frontend/src/api/entities.ts` — no signature change; the response shape adds the relevant order property inside `meta`.

Type metadata flowing to the frontend needs one new field per relation type:
`orderable: { outgoing: bool, incoming: bool }` (the frontend doesn't need to
know the enum; the booleans are enough — but the frontend always uses the
canonical property names `_order_out` / `_order_in` since those are stable). Add
to the metamodel-as-JSON serializer that already feeds the frontend.

Optional polish (defer if it bloats the ticket): factor the kanban DnD handlers
into a shared composable `useDragReorder` that both `KanbanView.vue` and
`RelationCards.vue` consume. Tempting but not required — keep an eye on the
duplication during implementation and decide then.

### 5. Analyze warning

In `dataentry/analyze.go`, add a new check `analyzeRelationOrder` that iterates
orderable relation types and emits warnings per parent entity for the *enabled*
side(s):

- two siblings share an order value (per side) → `relation.order.duplicate`
- one or more siblings have no order value → `relation.order.missing`

Warning codes match `analyze_*` finding-code convention so UIs can de-dup.

### 6. Why this shape

- Sticks to permissive-storage policy: missing/duplicate order values are warnings, never write rejections. The API sorts what it has, deterministically.
- Single-edge writes on reorder. Renumber is the rare-fallback path.
- No new endpoint, no parallel "reorder" API. The reorder is exactly one extra meta value in the existing PATCH.
- No new frontend dependency — native HTML5 DnD matches the existing kanban pattern.
- Orderability is declared on the relation type, not on the source entity type.
- `both` is composable from the two single-side mechanisms — no third code path.
- **Stable property names** (`_order_out`, `_order_in` regardless of mode) make metamodel evolution safe: promoting `outgoing` → `both` (or vice versa) requires zero data migration. The on-disk values keep their meaning.

**Files to modify:**

Backend:

- `internal/metamodel/types.go` — `OrderableMode` enum, `OrderPropertyOut` / `OrderPropertyIn` constants, `Orderable` field on `RelationDef`, helper methods.
- `internal/metamodel/loader.go` — validate enum value at load time; reject unknown strings.
- `internal/metamodel/validation.go` — verify no "unknown key" warning on the new field.
- `internal/entitymanager/order.go` — NEW, pure helpers above.
- `internal/entitymanager/entitymanager.go` — wire auto-assign and renumber into `CreateRelation` and the relation update path. Side selection driven by `OutgoingOrderProperty()` / `IncomingOrderProperty()`.
- `internal/dataentry/api_v1.go:690-734` — sort step on both outgoing- and incoming-side responses.
- `internal/dataentry/relations_modern.go` — pass order properties through; warning collection.
- `internal/dataentry/analyze.go` — new `analyzeRelationOrder` check, per-side.
- `internal/dataentry/api_v1_test.go` — table cases for AC2a/AC2b/AC4/AC6.
- `internal/entitymanager/order_test.go` — NEW, unit tests for helpers.
- `internal/entitymanager/relation_test.go` — AC7 cases across all three modes, plus AC10 (mode-change preserves values).

Frontend:

- `frontend/src/components/forms/RelationCards.vue` — native HTML5 DnD: `draggable` attribute + handlers conditional on the relevant side being enabled.
- `frontend/src/composables/useRelationReorder.ts` — NEW, side-agnostic midpoint compute + PATCH dispatch.
- `frontend/src/types/relations.ts` — extend `meta` typing to include optional `_order_out?: number` / `_order_in?: number`.
- Wherever metamodel metadata is exposed to the frontend (`internal/dataentry/api_v1.go` likely owns this serialization) — add `orderable: { outgoing, incoming }` per relation type.

Tests:

- `e2e/relations-order.spec.ts` — drag reorder e2e, covering outgoing and incoming sides. Reuse the Playwright `dispatchEvent('dragstart' …)` pattern if the kanban e2e already does that.

**Alternatives considered:**

- **Single `_order` for single-side modes, `_order_out` / `_order_in` only for `both`** — rejected per your feedback: a later mode promotion (`outgoing` → `both`) would orphan the `_order` values. Stable property names make metamodel evolution a zero-migration change.
- **`sortablejs` library** — rejected: the kanban view already uses native HTML5 DnD. Adding a competing library would split the pattern.
- **String LexoRank** — rejected: hand-editing markdown becomes opaque.
- **Dense integers + renumber every move** — rejected: every reorder rewrites N siblings, breaks AC3.
- **Separate `/reorder` endpoint** — rejected: extra surface for what is structurally a property update.
- **User-named order property** — rejected for v1: adds a metamodel key (`orderable.by`) and a UX choice. Backwards-compatible to add later.
- **Bool `orderable: true` (outgoing-only)** — rejected per your feedback: explicit enum supports the incoming and both cases at minimal cost.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

- **Order property from PATCH body** — must be a finite `float64`. Reject `NaN`, `+Inf`, `-Inf` with a `Hard 400` (malformed wire format). Accept negative and arbitrarily large values. Validation lives in `V1ResourceIdentifier` unmarshal.
- **Order property from on-disk markdown** — same parser path as other typed properties. Non-numeric or `NaN` value is logged and the relation sorts as "missing" — consistent with permissive-storage policy.
- **Metamodel `orderable` value** — enum allowlist: only `outgoing`, `incoming`, `both`. Anything else is a loader error at startup.

**Security-Sensitive Operations:**

- No new file-system access, no auth path changes. Reorder PATCH reuses the existing relation-update authorization.
- No injection surface: order values are numeric, never interpolated into queries or templates.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios (AC → tests):**

- **AC1** — `internal/metamodel/loader_test.go`: table cases `{"outgoing", "incoming", "both"}` load; `{"yes", "true", "OUTGOING"}` fail.
- **AC2a** — `internal/dataentry/api_v1_test.go::TestV1EntityRelations_OutgoingOrderableSorted`: fixtures `[_order_out=3, _order_out=1, _order_out=2, no _order_out]` → response `[1, 2, 3, missing]`.
- **AC2b** — `..._IncomingOrderableSorted`: same shape for the incoming-side response under `orderable: incoming`.
- **AC3** — `internal/dataentry/relations_modern_test.go::TestReorder_SingleWrite`: PATCH a midpoint, assert exactly one relation file rewritten. Covered for outgoing; mirrored test for incoming under `incoming` mode.
- **AC4** — `internal/entitymanager/order_test.go::TestSort_DuplicateValues`, `..._MissingValues`: assert deterministic output.
- **AC5** — `internal/dataentry/analyze_test.go::TestAnalyzeRelationOrder_DuplicatesWarn`, `..._MissingWarn`: assert warning code + severity for each enabled side.
- **AC6** — `internal/dataentry/api_v1_test.go::TestV1EntityRelations_NonOrderableUnsorted`: regression — type without `orderable:` preserves current behavior.
- **AC7** — `internal/entitymanager/relation_test.go::TestCreateRelation_AssignsOrder_Outgoing|Incoming|Both`: three table cases, one per mode.
- **AC8** — `internal/entitymanager/order_test.go::TestRenumber_OnCollapse`.
- **AC9** — `internal/entitymanager/relation_test.go::TestBothMode_SidesIndependent`: reorder on outgoing leaves `_order_in` untouched and vice versa.
- **AC10** — `internal/metamodel/loader_test.go::TestOrderable_ModePromotion`: load fixture under `outgoing`, switch metamodel to `both`, assert `_order_out` values still drive outgoing sort and `_order_in` is reported as missing (analyze warning) but does not corrupt anything.

**Frontend / e2e:**

- Component unit test (Vitest) for `useRelationReorder` midpoint compute.
- Playwright e2e: drag reorder on outgoing side; second test on incoming side under a `both` fixture.

**Edge Cases:**

- Single item in orderable list (drag is a no-op).
- Two items: drag swap → midpoint is `(a+b)/2`.
- Move to top: new value = `min - 1.0`.
- Move to bottom: new value = `max + 1.0`.
- Midpoint collapses → trigger renumber on that side only.
- All-missing values on a side → file order fallback; reorder writes only the moved relation, leaving siblings missing (analyze surfaces).
- Mixed-type polymorphic list → one ordered list, types interleaved.
- `both` mode: reorder on one side leaves the other side's property untouched.
- **Mode-change**: `outgoing` → `both` keeps `_order_out` values working; `_order_in` starts unset and is filled on next write per edge or surfaced via analyze warning.
- Concurrent reorder from two clients → last write wins per relation file; SSE refresh corrects. Acceptable for v1.

**Negative Tests:**

- PATCH with order value `"abc"`, `NaN`, `Infinity` → `400`.
- Metamodel with `orderable: "yes"` → loader error at startup.
- Reordering on a side that the metamodel does not enable → frontend never offers the drag handle; if PATCH arrives anyway, the write succeeds (it's just a property write on a non-reserved name) but no special semantics apply. Document this rather than reject.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

1. **Float precision collapse over time (medium).** Mitigation: automatic renumber when `NeedsRenumber` fires.
2. **Concurrent reorder from two SPA sessions (low).** Mitigation: last-write-wins is acceptable; SSE refresh corrects. Document as known behavior.
3. **Native HTML5 DnD UX rough edges (low).** No auto-scroll, no animated reflow. Acceptable — same trade-off the kanban view already lives with.
4. **Markdown manual edits break order (low).** Mitigation: tolerant rendering + analyze warnings.
5. **Stable property names ossify the wire/storage shape (low).** Mitigation: the names `_order_out` / `_order_in` are reserved via constants in one place. Renaming later is a one-line change at the constant plus a data migration — but the *point* of stability is to avoid that.
6. **Effort estimate — l (large).** Adding the `both` case widens the surface slightly: extra tests per AC, extra frontend conditional on the incoming side. Still mechanical.

## Documentation Planning

- [x] User-facing docs identified
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**

- [x] User guide / reference docs — new section "Orderable relations" under metamodel reference covering all three modes, with an explicit note that property names are stable across mode changes. Plus a UX note in the data-entry guide.
- [x] ~~CLI help text~~ (N/A: no command changes)
- [x] CLAUDE.md — short note about reserved properties `_order_out` / `_order_in` under "Rules for new code".
- [x] ~~README.md~~ (N/A: no project-level changes)
- [x] API docs — note the order properties in the `meta` map for orderable relations.

## Design Review

- [x] ~~Run `/design-review` before starting implementation~~ (N/A: pre-implementation review was performed via cranky-code-reviewer + go-architect parallel reviews after implementation; findings tracked as review-responses)
- [x] ~~All critical/significant findings addressed in plan~~ (N/A: addressed in review-response entities instead; all critical/significant RRs marked addressed before merge)

**Design Review Findings:** None during planning; addressed via review-response entities post-implementation.
