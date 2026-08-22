---
id: BUG-SDMD6O
type: bug
description: "scopedSortedEntities accumulates every entity of a type (bodies included) before handleV1ListEntities slices it to perPage. Measured 101 MB vs 1 MB on pgstore for 5000x20KB entities to render 50 rows. Invisible on memstore, which shares body strings."
why1: One list request holds ~100 MB of entity bodies to render a 50-row page.
why2: scopedSortedEntities returns []*entity.Entity for the whole type; pagination slices only after the pipeline returns.
why3: Sort, filter and free-text intersection genuinely need the full SET, so the full LOAD was assumed necessary along with it — but a list row renders properties, never the body.
why4: When EntityHeader landed (TKT-1ESTYJ) it was applied to the analyze path that motivated it; the list path has the same shape and was not revisited.
why5: "The default test backend (memstore) shares body strings rather than materialising them, so body-retention costs ~1 MB there and ~101 MB on a real backend. The regression is structurally invisible to the fast test path, so no unit test could have caught it."
title: GET /api/v1/<type> retains every entity of the type (bodies included) to render one page
priority: high
effort: m
status: backlog
---

## Symptom

`GET /api/v1/<type>` loads **every entity of that type, bodies included**, into
one slice, then slices it to `perPage` afterwards. Rendering 50 rows of a 5,000
-row type transfers and retains all 5,000 bodies.

This is the shape that made `_analyze` an OOM (TKT-1ESTYJ), on a more frequently
hit endpoint: the analyze fix removed whole-store retention, but the per-type
list path still retains a whole type.

## Measured

Real pgstore, 5,000 entities x 20KB bodies (~97 MB), one request:

| Retention | Heap |
|---|---|
| Whole entities (current) | **+101 MB** |
| Headers only | **+1 MB** |

A **100x** difference to render 50 rows. Cost scales with the largest single
type and with concurrency: N simultaneous list requests hold N copies.

Note memstore shows only +1 MB for the same probe — it shares body strings
rather than materialising them, so this defect is **invisible on the default
in-memory test path** and only appears against a real backend. That is why unit
tests did not catch it, and it is the same blind spot that hid the analyze bug.

## Root cause

`internal/dataentry/api_v1.go`, `scopedSortedEntities` (~:287). Both verdict
branches accumulate `*entity.Entity`:

```go
case rqr.AllowAll:
    for e, err := range a.Services().Store.ListEntities(ctx, store.EntityQuery{Type: typeName}) {
        entities = append(entities, e)      // whole entity, body included
    }
default:
    for e, err := range a.Services().Store.GraphQuery(ctx, *rqr.Query) {
        entities = append(entities, e)      // same
    }
```

`handleV1ListEntities` (~:592) paginates only after the pipeline returns:

```go
entities, err := a.scopedSortedEntities(r.Context(), typeName, query)
total := len(entities)
entities = entities[start:end]              // 50 of 5000, after loading all
```

The full set is genuinely needed *as a set* — sort, filter, and free-text
intersection all run across it, and `total` feeds the pagination envelope. What
is NOT needed is each entity's **body**: a list row renders properties, and the
row's body is never read.

## Why this is not the analyze bug again

Analyze retained the whole store; this retains one type. It is bounded by the
largest type rather than the whole dataset, so it is less severe. But that bound
is exactly what the scheduler leak (BUG-ZKK2UL) destroyed in production — it
inflated one type to 11k rows. The two defects compound the same way analyze and
the scheduler did.

## Fix direction

Sort/filter/count on **headers**, then load bodies only for the page.

`store.EntityHeader` / `ListEntityHeaders` already exists (TKT-1ESTYJ) and
carries id, type, properties and updated_at — everything the sort and property
filters read. The shape becomes:

1. `ListEntityHeaders` / a header-yielding `GraphQuery` for the ACL branch
2. filter + sort + free-text intersect over headers (`total` from the count)
3. `GetEntity`/`EntityQuery{IDs: pageIDs}` for the ~50 rows actually rendered

Blockers to resolve:

- **`GraphQueryer` has no header variant.** The ACL branch yields
`*entity.Entity`. Either add a header-returning graph query or accept bodies on
the gated path only.
- **`visibility` header gating already exists** (`FilterHeaders`,
`ScriptReader.ListEntityHeaders`) from TKT-1ESTYJ, so the ACL equivalence work
is largely done and asserted against `ListEntities`.
- **Does anything downstream read `.Content` from a list row?** Export
(`export.go`) shares `scopedSortedEntities`; a markdown export may legitimately
need bodies. That caller likely wants the whole-entity path retained, so the
header path must be opt-in per caller rather than a blanket swap.

## Acceptance criteria

1. Heap for one list request is flat in the type's entity count, not linear:
5,000 x 20KB renders within a few MB, not ~100 MB.
2. `total`, sort order, filtering, and free-text results are byte-identical to
today for every existing test.
3. ACL row-gating and field redaction are unchanged — asserted as equivalence
against the current path, not hand-written expectations (the TKT-1ESTYJ
pattern).
4. Export and any caller that genuinely needs bodies keeps them.
5. N concurrent list requests scale in page size, not in type size.

## Test plan

- **Retention test against a real backend** (pgstore, DB-gated like the existing
suite): assert heap growth for one list request is bounded well below the
dataset's body size. Must fail on current develop.
- **Explicitly not memstore** for that assertion — it shares body strings and
reports ~1 MB either way, which is what let this ship.
- Equivalence tests: same ids, same order, same `total` before/after, across
AllowAll / scoped / DenyAll verdicts.
- ACL equivalence asserted against `ListEntities` output, mutation-verified by
removing the gate and confirming failure.
