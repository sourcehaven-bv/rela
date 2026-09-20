---
id: TKT-UR4C8G
type: ticket
title: MIN_SEARCH_LEN=2 sits below pg_trgm's 3-character threshold, so the first mention search cannot use the index
kind: enhancement
priority: low
effort: s
status: backlog
---

## Problem

`MIN_SEARCH_LEN = 2` in `useMentionMenu.ts` is the point at which the `@`
mention menu starts searching. On the postgres backend the free-text path is
served by a GIN **trigram** index, and pg_trgm needs **3** characters to produce
a trigram.

So the very first search a user triggers — the 2-character one — cannot use the
index and falls back to a scan. Every subsequent keystroke can.

Surfaced during TKT-6MZ42J while answering the operator's question about query
counts and cost. It is **pre-existing** and not caused by that ticket; the
mention menu inherited the constant.

## What to decide

Either is defensible; the point is to decide deliberately rather than leave the
mismatch by accident:

1. **Raise `MIN_SEARCH_LEN` to 3.** One fewer request per typed query, and every
search that does run is index-served. Costs a little responsiveness: a user
typing a 2-character ID prefix waits for a third character.
2. **Keep 2 and accept the scan.** Defensible if the 2-character case is measured
to be cheap enough on realistic corpora — the result set is capped and the scan
may well be fast at the sizes that matter.

Option 2 needs a measurement to stand on, which is the actual work here.

## Where to measure

`docs/postgres-backend.md` and the root CLAUDE.md name the method: run
`rela-server -verbose` against `prototypes/perf/project` seeded by `rela dev
seed`, and read `Server-Timing` plus the one `request` log line per request.

Note the seeded perf project is the right target precisely because the tickets
corpus (~4k entities) is small enough that a scan may look free while being
genuinely expensive at 10x.

## Related

There is also no query-count budget test covering the free-text search branch.
The `storetest.Counting` budget pattern the root CLAUDE.md mandates for new read
paths ("assert the count is the same at 10 and 50 rows") would catch a future
N+1 on this path; it is currently unguarded. Worth doing in the same pass.
