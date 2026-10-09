---
id: BUG-1DWMYO
type: bug
title: 'Version sweep starves: edits after the first Batch settled rows are never captured'
description: 'The postgres and sqlite version sweeps selected settled rows whether or not they changed and deduped in Go after LIMIT Batch; with more than Batch settled unchanged rows the same rows filled every tick and newer creates/edits never got a version. Fix: a stored content_hash on live rows, written back by the sweep and cleared by a trigger, so the candidate query selects exactly what the dedup captures.'
priority: high
effort: m
why1: The sweep's candidate query returns the same oldest Batch rows every tick and newer edits are never selected.
why2: The WHERE clause selects every settled row (or every row with a version older than MaxStaleness) whether or not it changed; the change check is the Go-side content-hash dedup after the LIMIT.
why3: A skipped (deduped) row changes nothing the query keys on (updated_at, lv.created_at), so the next tick sees the identical first Batch rows.
why4: Tests used fewer rows than Batch; the sqlite drain fix covered only never-captured rows, not edits behind a full batch of older captures, and was not ported to pgstore.
why5: No conformance test drives the sweep with more settled rows than Batch, so the two backends' candidate queries were never held to a 'every change is eventually captured' contract.
prevention: 'Conformance tests on both database backends with a backlog larger than the sweep batch: never-captured rows drain, an edit to each hashed column behind a full batch is captured in one tick, and rows saved unchanged after a force-live purge neither re-capture nor block. The candidate query compares the same hash the dedup does, so it cannot select a row the dedup skips.'
started: "2026-10-08"
completed: "2026-10-09"
status: done
---

## Observed

On atlas (postgres, 825 entities) entities edited since 2026-10-01 have no
version history. `INC-003`, created by a webhook on 2026-10-06 and edited via
MCP on 2026-10-08, has no `entity_versions` row at all; 166 entities have none.
No error is logged: the sweep runs, examines 500 rows, and captures nothing.

## Cause

`selectCandidates` (pgstore) selects every row that has settled (`updated_at <
now - Idle`) or whose last version is older than `MaxStaleness`, ordered by
`updated_at ASC`, `LIMIT Batch` (500). The "is it changed?" check is the
content-hash dedup in Go, after the select. Once more than `Batch` rows are
settled and unchanged, the same oldest 500 rows fill every tick, and anything
newer is never reached.

sqlitestore fixed the never-captured half (order by `lv_created`, nulls first),
and its comment notes pgstore has the same starvation. An entity that already
has a version and is then edited still starves there when more than `Batch` rows
have an older, unchanged capture. The relation sweep has the same shape on both
backends.

## Fix

Live entity and relation rows get a `content_hash` column (pgstore migration
0020, sqlitedb v14). NULL means "not known".

- The sweep selects rows whose stored hash is NULL or differs from the current
lifecycle's latest version, or that have no version yet. This is exactly the set
the Go dedup would capture.
- After examining a row, the sweep writes back the hash it computed, guarded by
`xmin` (pg) or content equality (sqlite), so a concurrent write is never marked
clean.
- A trigger clears the stored hash when a hashed column changes. Writers need
no change, and a missed write path or an older binary cannot leave a stale hash.

A first attempt compared stored columns instead. Review showed rows that gate
marks dirty and the dedup skips (large integers, unchanged saves after a
force-live purge), which starve again; see the review responses. The review also
found that sqlite purge read type and face in swapped columns, so a force-live
tombstone carried the wrong hash; fixed here.
