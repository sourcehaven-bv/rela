---
id: TKT-1ESTYJ
type: ticket
title: 'Bound analyze memory: content-free entity headers, streaming analyzers, per-section issue caps'
kind: refactor
priority: critical
effort: l
status: done
---

## Problem

A single `GET /api/v1/_analyze` takes the server from **19 MB to 2,947 MB RSS**.
Three concurrent requests reached **6,264 MB** — it multiplies per request with
no bound. This is a remote OOM trigger reachable by any caller that can reach
the endpoint.

Reproduced locally against pgstore (20k entities, 1,974 MB of body content):

| Load | Peak RSS |
|------|----------|
| Idle | 19 MB |
| 1 x `_analyze` | **2,947 MB** |
| 3 x `_analyze` concurrent | **6,264 MB** |

Heap profile during the request — an unambiguous retention chain:

```
handleV1Analyze -> runAnalysis -> analyzeProperties
  -> visibility.ScriptReader.ListEntities -> pgstore.scanEntity
    -> pgx scanPlanString.Scan ......... 1.05GB (99.08% of live heap)
```

### Root cause

Analyzers drain the whole store into a slice/map and hold it across the scan
(`internal/dataentry/analyze.go:447`):

```go
entities := make([]*entity.Entity, 0)
for e, err := range svc.reads.ListEntities(ctx, store.EntityQuery{}) {
    entities = append(entities, e)   // retains EVERY entity, content included
}
sortStoreEntitiesByID(entities)      // ...because it needs them all at once, to sort
```

Two independent defects compound:

1. **Retention** — `[]*entity.Entity` / `map[title][]*entity.Entity` held across a
full scan. O(n) in entity count.
2. **Payload** — every retained entity carries its markdown body.
**`analyze.go` never reads `.Content` — zero references.** The 1.05 GB above is
entirely wasted bytes.

Same shape at `internal/dataentry/helpers.go:695` (`listAllFromStore`),
reachable from list rendering — must be checked before this is closed.

Contrast with `entitymanager.collectAllIDs`, which runs the same full scan but
keeps only `e.ID`: it streamed 2 GB at **53 MB** RSS. Retention, not scanning,
is what kills the process.

### Why this went unnoticed

`_analyze` is fine on a small project. It became fatal when a separate scheduler
bug (see "Related") inflated one entity type to ~11k rows. Table size converts
directly into RSS.

## Scope

**In scope**

- **(A) Content-free entity reads.** New `store.EntityHeader` (ID, Type, Properties,
UpdatedAt — *no* Content) + `ListEntityHeaders`, as an optional store capability
type-asserted like `store.Formatter` / `HistoryReader`, with a generic fallback
that drops content client-side. pgstore projects the column away so the bytes
never leave the DB.
- **(B) Streaming analyzers.** `analyzeProperties` / `analyzeCardinality` emit issues
inside the scan; sort *issues* at the end instead of entities. Memory becomes
O(issues), not O(entities).
- **(C) Per-section issue cap.** Hard cap of 100 issues per analyzer section. Fetch
the 101st purely to detect overflow, report 100, and set a `Truncated` flag on
the section. No `?limit=` opt-out.
- Audit `listAllFromStore` (`helpers.go:695`) for the same defect.
- Give `analysisIssueCounts` a real count-only path (see Risks).

**Out of scope**

- Migrating list / search / kanban-card surfaces to `EntityHeader`. Deliberate: those
are the natural follow-ups and the reason (A) is a typed capability rather than
a local fix, but each is its own verifiable migration. Separate tickets.
- Caching / precomputing analysis, or moving it to a background job. That is the real
fix for the per-request multiplier (3 requests = 6.2 GB) — file as a follow-up.
- ACL or rate gating on `_analyze`. Worth doing (6 full scans per request is expensive
regardless) but orthogonal; CLAUDE.md permits a gate for cost reasons as long as
it is not justified as concealment.

### Why a new type instead of an OmitContent flag on EntityQuery

A bool on the shared query type returns something that *is* an `entity.Entity`,
satisfies every interface, and lies. There are **337 `.Content` reads across 12
packages**. The sharpest example: `computeEntityETag` (`api_v1.go:1828`) hashes
`e.Content` — hand it a content-omitted entity and it returns a well-formed ETag
for the wrong bytes, silently breaking conditional requests and caching.

`computeEntityETag(*entity.Entity)` will not compile against an `EntityHeader`.
The mistake becomes unavailable rather than merely discouraged — the same
reasoning that motivated `PatchEntity` replacing read-modify-write.

## Acceptance criteria

1. `GET /api/v1/_analyze` over 20k entities x ~100 KB bodies stays under **150 MB**
peak RSS (from 2,947 MB). Verified by the repro harness, RSS sampled at 20 ms.
2. Peak RSS for `_analyze` is **flat in body size** — 20k entities with 100 KB bodies
costs the same as 20k with empty bodies.
3. **3 concurrent** `_analyze` requests stay under **300 MB** combined (from 6,264 MB).
4. Analyzer output is **unchanged** below the cap: same issues, same order, for a
fixture project with fewer than 100 issues per section.
5. A section with more than 100 findings returns exactly 100 issues with `Truncated`
true; the UI shows the list is truncated. No exact total is displayed.
6. `analyzeDuplicates` still detects duplicates correctly — it cannot stream (a title
is not known to be duplicated until its second occurrence), so it groups headers
first and caps at issue-emission.
7. A store backend without the capability still works via the generic fallback;
`storetest` conformance covers `ListEntityHeaders` for any implementation.
8. No `EntityHeader` reaches a `.Content` consumer — enforced by the type, verified by
compilation.

## Test plan

- **Memory regression test** — seed N entities with large bodies, run `_analyze`,
assert peak heap via `runtime.ReadMemStats` stays under budget. This is the test
that would have caught the bug; it must fail against current `develop`.
- **Golden output test** — fixture project under the cap; assert analyzer output is
byte-identical before/after (criterion 4).
- **Cap boundary tests** — sections with exactly 99, 100, 101, and 500 issues.
Assert 101 gives 100 issues + `Truncated`; 100 gives 100 issues + not truncated.
This pins the off-by-one in the fetch-101 logic.
- **Duplicates correctness** — duplicate groups spanning the first and last rows of a
scan, to prove grouping is not broken by streaming changes elsewhere.
- **storetest conformance** — `ListEntityHeaders` returns the same ID/Type/Properties
set as `ListEntities`, with empty Content, across fs / mem / pg.
- **Fallback path** — a store *without* the capability yields identical headers.

## Risks

- **`analysisIssueCounts` (dashboard) silently degrades.** It calls `runAnalysis` and
discards everything but the totals; with sections capped at 100 its count
becomes "at least 100" rather than a true total. Mitigation: give it a real
count-only path — it should never have built full issue detail to produce a
number. In scope.
- **Fallback is correct but not fast.** On a backend without projection the bytes still
cross the wire; only the retention improves. Acceptable — pgstore is where the
production pain is — but do not describe the fallback as bounding I/O.
- **Cap changes semantics.** Analysis becomes partial above 100 per section. Surfaced
via `Truncated` in the UI, so this is a visible bound, not a silent one.
- **`EntityHeader` is a new public store surface.** Keep it minimal; adding Content
later would defeat the purpose.

## Related

- Scheduler amplification bug (separate ticket): a failed scheduled task never stamps
last-run, so the `!recorded` branch short-circuits before `IsDue` and the task
re-fires every tick — *every* schedule kind, not just `day`. This is what
inflated the entity table and made `_analyze` fatal. Neither bug alone is
sufficient.
- `entitymanager.collectAllIDs` (`core.go:207`) runs a full-content scan per entity
create: 0.74 s uncontended to 22 s under 80-way concurrency.
Latency/availability, not memory — separate ticket, and a candidate consumer of
`ListEntityHeaders`.
- `pool_max_conns` in the DSN is consumed by pgxpool but passed verbatim to the
change-feed listener's raw connection, which fails with `FATAL: unrecognized
configuration parameter`. Cross-process events silently degrade whenever an
operator tunes the pool. Separate minor ticket.
