---
id: TKT-HGE4KW
type: ticket
title: 'Deleted-relation history: disambiguate id-reuse across lifetimes (relation tombstone)'
kind: enhancement
priority: medium
effort: m
status: done
---

## Problem

Reading the history of a **deleted** relation silently returns only the MOST
RECENT lifetime of a reused `(from, rel_type, to)` key — older deleted lifetimes
of the same key exist in `relation_versions` but are unreachable and invisible.

`pgstore.recordIDForKey` (relation_version.go), when the live row is gone, picks
"the lineage with the highest `max(vseq)` whose final row matches this key":

```sql
SELECT rv.rel_record_id
FROM relation_versions rv
JOIN (SELECT rel_record_id, max(vseq) AS vseq FROM relation_versions GROUP BY rel_record_id) latest
  ON latest.rel_record_id = rv.rel_record_id AND latest.vseq = rv.vseq
WHERE rv.from_id=$1 AND rv.rel_type=$2 AND rv.to_id=$3
ORDER BY rv.vseq DESC LIMIT 1
```

**Scenario.** `(A, blocks, X)` is created, edited, deleted → lineage
`rel_record_id=7`. Later a completely unrelated `(A, blocks, X)` is created and
deleted → lineage `rel_record_id=99`. A history read of the deleted key returns
99 only; lineage 7's history is orphaned — no way to navigate to it, and no
signal that a prior deleted relation with this key ever existed.

## Why entities don't have this

Entity history solves id-reuse with the recursive-CTE `[lo,hi)` vseq fencing
keyed on `prev_id` rename rows (version.go "ID reuse and vseq fencing"), plus
the fact that a live entity id is unique at any instant. Relations have NO
analog: the composite key is the only handle and it is ambiguous across time.
This was a write-side design; the gap is on the READ side, which is why it
wasn't caught in the TKT-92JL8P design review (the reviewers focused on
lineage-MERGE, not lineage-HIDE — this is the dual: lineages are correctly kept
separate but the older one becomes unreachable).

## Fix direction

The infrastructure already exists: `writeRelationTombstone` / `manifest.go`
(from the change-feed work, FEAT-NJ9FEN) durably records deleted relation
triples. A deleted-relation history read should be able to:

1. **Enumerate all past lifetimes** of a `(from,type,to)` key (return a list of
`rel_record_id`s, newest-first, each with its create/delete span), not just
silently collapse to the newest.
2. Let a caller select a specific lifetime (e.g. `--lifetime N` on the CLI, a
picker in the UI) or default to newest with a "N earlier deleted lifetimes
exist" affordance.

Decide whether to lean on the existing tombstones or add a
`relation_versions`-native "lifetime index" (the version rows already carry
enough: each lineage's first `create`/last `delete` bounds a lifetime).

## Scope

- No data loss today — the older history is retained, just unreachable. This is
a **completeness/correctness-of-read** bug, not a durability bug. Medium, not
critical: it only bites when a key is genuinely deleted-and-recreated, and the
common case (one lifetime) is correct.
- Out of scope: the same reasoning for entities (already fenced).

## Origin

Surfaced in the TKT-92JL8P follow-up review discussion — the user's "tombstone
for that case" intuition was exactly right; it's the read-side dual of the
lineage-merge hazard the design review closed on the write side.
