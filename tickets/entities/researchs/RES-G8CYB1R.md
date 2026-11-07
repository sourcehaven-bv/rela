---
id: RES-G8CYB1R
type: research
title: Set reconciliation for rela-to-rela sync
summary: Compared change feeds, Merkle buckets, range-based reconciliation, IBLT, Minisketch and Bloom filters; recommends range-based reconciliation with an additive SQL fingerprint.
status: done
---

## Problem

Two rela instances must find out which rows in a shared scope were added,
changed or are gone, without the server keeping state per reader. The answer
must respect row-level ACL (a hidden row looks the same as a deleted one) and
field-level `visible:` redaction.

## Context

- `_history` works on one entity at a time and lags: the version sweep captures
a version only after the entity has been idle.
- The SSE `_events` stream reports only which types changed, by design
(TKT-3A1200).
- A `since` cursor on the list endpoints could use pg's `rela_seq` and the
`deletions` table. It misses rows that enter or leave the scope without a write
(a role grant, a `related(...)` condition that flips). Reporting "no longer
visible" would need state per reader, and tombstones leak the ids of rows the
reader could never see.
- pgstore keeps a lazy `content_hash` on live rows (migration 0020). It is NULL
after every write until the sweep fills it.
- The entity ETag hashes only what the reader may see (BUG-J01V0Q).

## Options

Each option reconciles pairs of (address, view hash), so a changed row shows up
as the same id with two hashes.

| Option | Pros | Cons |
| --- | --- | --- |
| Change feed with `since` plus a periodic manifest | Low latency | Commit-order trap on `rela_seq`; needs a manifest pass anyway; tombstones leak ids |
| Bucketed hashes (Merkle tree) | Simple | Fixed buckets; a busy range makes it dig deep |
| Range-based reconciliation (Negentropy) | Fingerprint of a range is one SQL aggregate under the ACL predicate; no estimate of the difference; one round trip when nothing changed; about 4 to 5 round trips for 100,000 rows | Several round trips when there are many differences |
| IBLT or rateless IBLT | Few round trips | Server builds the table over the whole scope on every request; plain IBLT needs a size estimate |
| Minisketch (Erlay) | Close to the size limit | Expensive to decode; fixed-size elements |
| Bloom filter | Small | False positives make rows go missing silently |

## Recommendation

Range-based reconciliation with an additive fingerprint (sum of element
fingerprints, fixed width, computed in SQL). The server stays stateless; the
client keeps its id set, its bases and nothing else. Use the stored canonical
hash for types where the reader has no redacted fields, and compute the hash of
the redacted projection in Go, one page at a time, for the rest. Drop the change
feed from the sync design; `_events` only triggers an early run.
