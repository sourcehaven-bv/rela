---
id: TKT-0IGI4V
type: ticket
title: 'Flush-on-author-change: synchronous pre-edit version capture at author boundaries (pgstore)'
kind: enhancement
priority: medium
effort: m
status: backlog
---

Split out of TKT-ZIRMGM per design review (2026-07-24, RR-K781MZ): the
attribution fix (last_edited_by columns + sweep stamping) ships first; this
ticket adds the author-boundary flush so two DIFFERENT authors editing the same
entity/relation within one debounce window (default 5m) never collapse into one
version attributed only to the last author.

## Behavior

In pgstore `UpdateEntity`/`UpdateRelation`: when the incoming
`store.Attribution` differs from the row's `last_edited_by_*` AND the live
content hash differs from the lineage's latest version hash, synchronously
insert a version of the PRE-edit state attributed to the PREVIOUS author, then
apply the write. Same-author bursts keep the debounce; author boundaries always
fence a correctly-attributed version. Flush on ANY attribution change (user OR
tool differ).

## Pinned semantics (from TKT-ZIRMGM design review — treat as requirements)

- **Atomicity (RR-VG4BPJ):** the probe ("author differs AND hash differs") and the version INSERT run inside the update's own transaction, after the row is locked, so decision+insert are atomic with the write they fence. Under `store.Tx` the outer tx already holds `writeAdvisoryLockKey`.
- **Dedup backing (RR-4OJAC1):** sweep dedup is SELECT-then-INSERT under the sweep advisory lock; a flush insert outside it needs either (a) acquiring `sweepAdvisoryLockKey` for probe+insert (restores purge's mutual-exclusion assumption; weigh write-latency of a blocking lock) or (b) a partial unique index + ON CONFLICT DO NOTHING. Decide in planning.
- **Op choice (RR-MMDQ3N):** reuse the sweep's delete-fenced lineage probe (the two-LATERAL `lvc`/live-lineage logic in `sweep.go`) for BOTH the dedup hash AND create-vs-update, so a flushed version is indistinguishable from a swept one. An entity never yet swept flushes as `create`.
- **Purge safety (RR-MZ4PPG):** flush must snapshot the pre-edit state FROM THE ROW inside the update tx, never from caller-supplied memory — that's what guarantees a post-`--force-live`-purge flush carries the already-tombstoned live hash and is dedup-suppressed instead of resurrecting purged content. Document this reasoning in the code.
- **Never a rename marker (RR-MORL7M):** flushed rows always use op ∈ {create, update} with empty PrevID / PrevFrom / PrevTo.
- **Testable seam (RR-12HJ4K):** the flush decision is a pure function (probe result + incoming attribution → decision) unit-testable without the sweep ticker; DB-gated integration tests drive two updates with different `WithAttribution` ctxs and assert the synchronous pre-edit version.

## Depends on

TKT-ZIRMGM's migration (`last_edited_by_*` columns) and `store.Attribution` ctx
carrier being merged.
