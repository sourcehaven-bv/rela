---
id: BUGA-IYFBQI
type: bug-analysis-checklist
title: 'Analysis: Multi-enum list filter renders a native listbox and never matches any row'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Reproduction

- [x] Bug reproduced locally
- [x] Minimal reproduction steps documented
- [x] Environment/conditions noted

Reproduced at the HTTP boundary with a scratch test against
`app.handleV1ListEntities` (via the existing `runListFilter` helper), seeding
two tickets with a list-typed property:

```go
seedEntity(app, &entity.Entity{ID: "TKT-a", Type: "ticket",
    Properties: map[string]any{"tags": []any{"urgent", "blocker"}}})
seedEntity(app, &entity.Entity{ID: "TKT-b", Type: "ticket",
    Properties: map[string]any{"tags": []any{"later"}}})
```

Observed:

| Query | Got | Want |
| --- | --- | --- |
| `filter[tags]=urgent` | `[]` | `[TKT-a]` |
| `filter[tags][in]=urgent,later` | `[]` | `[TKT-a, TKT-b]` |
| `filter[tags][ne]=urgent` | `[TKT-a, TKT-b]` | `[TKT-b]` |

**The `ne` result is worse than the reported symptom.** The reported bug is
"returns nothing"; `ne` returns the entity it was asked to EXCLUDE. A filter
meant to narrow silently includes a non-matching row — a wrong-answer bug, not
just an empty-result bug. Any UI or saved view relying on `ne` over a list
property is showing incorrect data today with no visible error.

Environment: default (fs) build, `newTestAppV1` harness, no postgres needed —
this path is in-memory only.

The scratch test was removed after confirming; the permanent version is the
`adds-measure` deliverable (`AM-list-property-filter-any-element`).

## Root Cause

- [x] Immediate cause identified (why1)
- [x] Contributing factors found (why2-3)
- [x] Systemic cause explored (why4-5)

Recorded in full in the bug's `why1`–`why5` and the "Long-term fix" section of
BUG-AMK38R. Summary: `applyV1Filters` flattens every property value with
`fmt.Sprintf("%v", propVal)` (`api_v1.go:1837`) before comparing, so a list
becomes `"[urgent blocker]"` and no per-element comparison ever happens.

**Key finding added during analysis — the inconsistency is *within* the same
package.** `internal/dataentry` already contains a correct implementation:

- `propertyContains` (`helpers.go:207-226`) handles `string`, `[]string` and
`[]any` properly, using `fmt.Sprintf` only as the scalar fallback.
- It backs `applyFilters` (`helpers.go:248-283`), which applies the
**static, config-authored `filters:`** entries.

So a list-typed filter authored in `data-entry.yaml` as a static filter matches
correctly, while the *identical* predicate arriving as a `filter[...]` query
param from the UI matches nothing. Same package, same concept, two behaviors.

That brings the count of parallel implementations of "does this property value
match this string" to **four**: `filter.matchList`, `propmatch.equalsTarget`,
`dataentry.propertyContains` (all correct) and `applyV1Filters` (wrong).

## Fix Planning

- [x] Fix approach determined
- [x] Regression test planned
- [x] Related areas checked for similar issues

### Approach

**Step 1 — backend.** Route every operator branch in `applyV1Filters` through a
single value-comparison seam instead of the `propStr := fmt.Sprintf("%v", …)`
flattening. `propertyContains` is the natural basis: it is in this package,
already correct, and already the definition used by static filters — so adopting
it makes the dynamic path agree with the static one rather than introducing a
fifth rule.

Semantics (matching `filter.matchList`):

- `eq` / `in` / `contains` → ANY element matches
- `ne` → NO element matches
- ordered ops (`lt`/`lte`/`gt`/`gte`) → unchanged; they remain scalar-only, and
a list operand keeps today's behavior rather than gaining new meaning in a
bugfix

Scalar behavior must be byte-for-byte unchanged. Only the list case changes, and
it currently cannot match at all, so there is no working behavior to preserve.

**Step 2 — frontend.** Render `TagSelect` for `widget === 'multi-select'` in
`FilterBar.vue`, and emit `op: 'in'` so the wire form is `filter[p][in]=a,b`.
Reuse the `schemaStore.resolveOptionLabels` map FilterBar already builds
(`FilterBar.vue:101`) — the same one `MultiSelectWidget` passes to `TagSelect`.
`getMultiSelectValues` becomes the string↔array adapter.

**Step 3 — out of scope here.** Convergence onto one matcher is TKT-UTJ24Z,
sequenced after TKT-HFEKVN. Rationale for splitting is in BUG-AMK38R.

### Regression tests

1. HTTP-level table test over `eq`/`ne`/`in`/`contains` against a list-typed
property — the exact matrix reproduced above, including the `ne` wrong-answer
case. This is the `adds-measure` deliverable.
2. Direct unit test for `filter.matchList`, which has none today despite being
the reference implementation.
3. Frontend: assert the multi-enum control renders `TagSelect` (not a native
`<select multiple>`) and emits `op: 'in'`. Note the existing test at
`FilterBar.test.ts:263` asserts only `wrapper.find('select').exists()` for a
*scalar* enum and would still pass — it must not be treated as covering this.

### Related areas checked

- `sections.go:32,39,328` — `fmt.Sprintf("%v")` used for display/grouping, not
equality. Not affected.
- `api_v1.go:1934-1935` — sorting comparison, scalar-oriented; list sorting is a
separate question and explicitly not in scope.
- `api_v1.go:2175-2176` — conflict-diff display strings. Not a filter.
- `helpers.go:224` — the correct scalar fallback inside `propertyContains`.
- Store-side pushdown: not involved. `api_v1.go:436` confirms relation-title
pushdown is future work, and this list path is in-memory only, so no SQL
predicate needs to change.
