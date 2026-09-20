---
id: BUG-SDMD6O
type: bug
description: "scopedSortedEntities accumulates every entity of a type (bodies included) before handleV1ListEntities slices it to perPage. Measured 101 MB vs 1 MB on pgstore for 5000x20KB entities to render 50 rows. Invisible on memstore, which shares body strings."
why1: One list request holds ~100 MB of entity bodies to render a 50-row page.
why2: scopedSortedEntities returns []*entity.Entity for the whole type; pagination slices only after the pipeline returns.
why3: Sort, filter and free-text intersection genuinely need the full SET, so the full LOAD was assumed necessary along with it — but a list row renders properties, never the body.
why4: When EntityHeader landed (TKT-1ESTYJ) it was applied to the analyze path that motivated it; the list path has the same shape and was not revisited.
why5: "The default test backend (memstore) shares body strings rather than materialising them, so body-retention costs ~1 MB there and ~101 MB on a real backend. The regression is invisible to a HEAP probe on the fast test path — but not to a body COUNT, which is backend-independent and is what now pins it."
prevention: "storetest.BodyWatch counts bodies served to a read path, so 'this pipeline read markdown it will not render' is an exact, backend-independent assertion rather than a heap measurement only a DB-gated job can make. It is the third sibling of Counting (round-trips) and Breadth (batch width); a new collection read path pins its body cost the same way CLAUDE.md already requires a Counting budget test."
title: GET /api/v1/<type> retains every entity of the type (bodies included) to render one page
priority: high
effort: m
status: review
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


## Resolution

The retention defect itself was already fixed by **TKT-1U8XYN**, which landed
after this bug was written. `scopedHeaders` reads `store.EntityHeader` rows on
every verdict branch, `listpushdown.go` serves a page straight from the store,
and pgstore projects the content column away in SQL. Verified by measurement,
not by reading: a list over a 50-row type reads **0 bodies**.

What this change adds is the part that was genuinely missing — **the pin**, and
one real regression the pin found.

### The pin (`storetest.BodyWatch`)

The ticket's test plan called for a DB-gated heap assertion, explicitly not on
memstore. That instrument was reconsidered and replaced, for the reason the
ticket itself supplies: memstore is blind to body retention, so a heap test
there cannot fail — and a pgstore-only test does not run in the default
`go test ./...`. The defect would simply have been reintroduced somewhere the
assertion was not watching.

`BodyWatch` counts bodies SERVED instead of weighing them. That is the same
property, asked in a form every backend can answer: exact, thresholdless, and
it runs everywhere. It is the third sibling of `Counting` (round-trips) and
`Breadth` (batch width) — each measures a cost the other two cannot see.

Mutation-verified against the exact defect. Reverting the AllowAll branch to
`ListEntities` fails with *"list of 50 rows read 50 bodies, want 0"* — on
**memstore**, the backend a heap probe reports ~1 MB on either way.

### The regression the pin found

Requirement 4 asked whether any list row legitimately needs a body. One does,
and it had silently broken: a list export with an `export_render:` Lua script
reaches `row.content`, and since collection reads became content-free it was
receiving the **empty string for every row**. The export still returned 200 and
still emitted a document; only the bodies were gone. Confirmed by checking out
`bb8d3a144~1`, where the same test passes.

Fixed with a `loadBodies` seam called on the override path only, after the ACL
scope, the field-redaction pass and the cap — so it loads at most
`listExportCap` bodies, for rows that already survived every gate. The
built-in column table renders columns and stays content-free, pinned in both
directions.

The existing override tests could not have caught this: the shared
`fakeScriptEngine` records row IDs only, so the rows arrived in the right order
with the right ids and every body empty.
