---
id: DEC-IYHLNF
type: decision
title: 'Lua read bindings: uniform opts table, bounded results, cursor paging'
context: The three Lua read bindings (list_entities, search, get_relations) grew independently with three unrelated signatures; two are unbounded and all three swallow iterator errors. Fixing paging in one binding alone would have produced a fourth convention, so the whole read surface is decided together.
consequences: 'Breaking change to three documented bindings (backwards compat explicitly waived). The ACL row gate moves from post-filter to a store.GraphQuery pushdown — but the pushdown replaces the ROW gate ONLY; field redaction still runs per row, or the #1188 finding returns. Staged: shape + hard bound now, cursor + each_entity once store.GraphQueryer grows Limit/Cursor. Until then a type over 2000 rows is unreachable, accepted because the largest in-tree type is 933.'
date: "2026-07-29"
status: accepted
---

## Context

The three Lua read bindings grew independently and share no convention:

| Binding | Signature | Bound |
|---|---|---|
| `list_entities(type, filter?)` | positional, filter is a string DSL | none |
| `search(query, limit?)` | positional, limit is arg 2 | 20 |
| `get_relations(opts?)` | options table | none |

Two of the three are unbounded (TKT-YWDGZD), and all three `break`-and-discard
on iterator error (TKT-FVQ4 — confirmed empirically 2026-07-27: `get_relations`
returns **0 rows AND no error** on a failed query, indistinguishable from "no
such edges").

Fixing paging in `list_entities` alone would have produced a fourth convention.
This decision covers the whole read surface.

## Decision

**1. One shape: `f(required, opts?)`.** Every read binding takes its required
argument(s) positionally and everything else in an options table, where `limit`
and (later) `cursor` mean the same thing everywhere.

**2. Always bounded. No unbounded spelling.** The unbounded path being the
DEFAULT is the defect; an opt-in bound leaves the footgun as the path of least
resistance, and an unbounded *option* re-creates the problem for whoever reaches
for it. `limit = 0` is NOT "everything" (this diverges from
`store.ListEntitiesPage`, where 0 does mean unbounded — documented at the
binding).

**3. Cursor paging, never offset.** Offsets over a mutating graph silently skip
and duplicate rows. Keyset cursors are what `pgstore.ListEntitiesPage` already
implements; this adopts the existing contract.

**4. `total` is NOT returned.** Computing it means a second query (`GraphCount`)
on every list call, paid by callers that ignore it. It is also redundant under a
cursor (`next_cursor == nil` answers "am I done?"), and `GraphCount`'s `total`
deliberately ignores the ACL predicate, making it a count oracle over hidden
rows (RR-SSPCCI). Scripts needing a count get `count_entities`, which MUST count
post-gate.

**5. Iterator errors raise.** A store failure must not present as a short or
empty list (TKT-FVQ4).

## The numbers

| Role | Value | Reasoning |
|---|---|---|
| **Max, all stages** | **2000** | Upper end of the 500–2000 batch band. ONE number for "most you can pull at once", pre- and post-cursor |
| Default page, post-cursor | 1000 | Modal default across ecosystems (`find_in_batches`, psycopg2 `itersize` 2000, typical JDBC fetch 500–1000); one round trip for every type in the graph today |

2000 is deliberately NOT `listExportCap`'s 5000. That cap bounds a rendered
export (memory in a converter); this bounds a script's per-call fetch. Verified
no in-tree type exceeds 2000 — largest is 933 (review-responses) — so a single
2000 ceiling breaks nothing today and needs no pre/post-cursor split.

**These are engineering rules of thumb, not derived values.** The cost curve is
U-shaped with a wide flat bottom: below ~100 per-round-trip overhead dominates;
above ~5000 you hold a large result while doing per-row work and lose the point
of batching. Between 500 and 2000 the difference rarely appears in a profile.

**Explicitly NOT 25/100.** Those are `parseV1Pagination`'s HTTP values, right
for a browser-facing API bounding response size and latency. A Lua script runs
**in-process** next to the store — no HTTP response to bound. Smaller pages mean
MORE database round trips, so web pagination conventions argue the wrong
direction here. This was considered and rejected; do not "harmonize" the two
surfaces without re-deriving why.

Declared as `var`, not `const`, so tests can lower them without seeding
thousands of rows (same rationale as `listExportCap`) — and so the next person
tunes on evidence rather than treating 1000 as sacred.

## Staging

Store-side paging does not exist: `store.GraphQueryer` has no `Limit`/`Cursor`
(verified against `develop` 2026-07-29; nothing merged, nothing open).

- **Stage 1 (this PR):** uniform `opts` shape, hard 2000 ceiling enforced by
early-exit on the iterator, `count_entities`, error-raising, loud rejection of
bad `limit`. **`cursor` is ABSENT, not inert.** An accepted-but-ignored cursor
produces scripts whose paging loop silently never advances; an offset-backed one
corrupts under concurrent writes. A missing parameter is a clear error, a fake
one is a trap.
- **Stage 2 (blocked on the store):** `cursor` + `each_entity` auto-paging
iterator, so the safe path is also the shortest path. Ceiling becomes a page
size.

Interim gap accepted: a type over 2000 rows is unreachable until stage 2.

## Consequences

- Breaking change to three documented bindings. Backwards compatibility
explicitly waived by the user (2026-07-28).
- The ACL must move from post-filter (`PolicyReader.Filter`) to pushdown
(`store.GraphQuery` via `acl.Request.ReadQuery`) so a bounded read returns a
FULL page of visible rows rather than a page filtered down to fewer.
**Critically: the pushdown replaces the ROW GATE ONLY.** `Filter` also performs
field redaction, which `GraphQuery` cannot express — dropping it wholesale would
reintroduce the #1188 CISO finding (RR-1W1G6K).
- `count_entities` needs an ACL-scoped count; `store.CountEntities` takes an
`EntityQuery`, which cannot express the ACL predicate. Second store-side ask.
- Gate errors change from silent per-type drop to a raised error (RR-4DUSO1) —
accepted as an improvement, consistent with point 5.
