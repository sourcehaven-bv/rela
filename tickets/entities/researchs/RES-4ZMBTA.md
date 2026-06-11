---
id: RES-4ZMBTA
type: research
title: How should pgstore propagate writes across processes (multi-writer change feed)?
summary: 'Recommend Option A (LISTEN/NOTIFY): payload carries kind:op:id so notifications ARE targeted store.Events on the happy path, with a seq>lastSeen catch-up backstop on reconnect + safety ticker for durability (recoverable, not best-effort). Skew on the backstop only: overlap-window + idempotent re-snapshot now, xid8+xmin-horizon as the exact upgrade later. Search needs no feed (always-live); the consumer side (data-entry SSE / MCP) isn''t wired to pgstore yet — planning decides which consumers subscribe.'
status: done
---

## Problem

`pgstore` (TKT-M8400) is single-writer: its `store.Watcher` emits events only to
in-process subscribers. Under multiple writers against one database, one
process's committed writes are invisible to another process's derived state,
which silently drifts from the DB. TKT-WZYWM9 asks: how should pgstore propagate
committed writes across processes — and recover events missed while
disconnected? The schema already carries a global monotonic `seq` (sequence
`rela_seq`) + timestamps on every row, added in TKT-M8400 to enable watermark
catch-up without a migration.

## Context

Grounded in the codebase (file:line verified):

- **Event payload is identity-only.** `store.Event` carries `Op`, `EntityType`,
`EntityID`, `RelationType`, `From`, `To` (store.go:257-276) — NO data, NO seq.
Delivery is lossy + unordered (full buffers drop). A cross-process feed can
carry those same identity fields; consumers re-snapshot from them.
- **Zero production subscribers to pgstore's watcher today.** The data-entry SSE
feed (dataentry/watcher.go) and MCP watcher (cli/mcp_wiring.go:192-237) only
attach to fsstore's *file* watcher via a `StartWatching()` interface that
pgstore does NOT implement; data-entry broadcasts entity events inline from its
write handlers, not from store events. => wiring pgstore into those consumers is
part of the work, not just building the feed.
- **Search holds no derived state** — pgstore SearchBackend.Search queries the DB
live each call (search.go:51-91); EntityPut/EntityDelete are no-ops. Search is
always current regardless of writer and needs NO feed.
- **`updated_at` is NOT a usable per-row watermark.** It's set at write time
(transaction start, `now()` is txn-stable), so it has the SAME commit-order skew
as `seq` and is coarser. Useful only as a global staleness flag
(`LastModified()` already does `max(updated_at)`), not for per-change ordering.
- **`seq`** is a global monotonic BIGINT stamped on every write across the 3
tables (0001_init.sql); queryable `WHERE seq > $N`. Written, never read yet.
- **Connection model:** the store holds the pool only as the narrow `DBTX`
interface (no `Acquire`). A dedicated LISTEN connection needs the connection
seam widened, or a `*pgxpool.Pool`/Listener handed to `pgstore.Open`.
- pgx/v5 (v5.9.2) already a dependency; no LISTEN/NOTIFY code yet.

## Options

### Option A — LISTEN/NOTIFY with the changed ID in the payload + seq catch-up backstop
Producer: in the write txn, `pg_notify('rela_changed', '<kind>:<op>:<id>')`
(e.g. `entity:updated:FEAT-3`; well under the 8000-byte limit; delivered only on
commit). Consumer holds a dedicated listening connection and, on each
notification, parses the payload and emits the matching targeted `store.Event`
directly — no query needed on the happy path. DURABILITY BACKSTOP: because
NOTIFY has no replay (a process that's down/reconnecting misses notifications),
also run a `WHERE seq > $lastSeen` catch-up on startup, on every reconnect, and
on a slow safety ticker, advancing a `seq` watermark. The payload is the fast
path; the catch-up makes it recoverable.
- **Pros:** near-instant, *targeted* per-change events (not just "refresh");
recoverable across disconnects; handles deletes precisely (payload says which
id). **Cons:** dedicated LISTEN connection per process; reconnect logic;
connection-seam change; skew handling needed on the backstop path.
- **Effort:** Small–Medium.

### Option B — Poll on the seq watermark
Periodic `WHERE seq > $lastSeen`. Simple, no connection change, latency = poll
interval, constant idle load. The catch-up half of A without the NOTIFY speedup.
Effort Small–Medium.

### Option C — Logical replication / CDC (pglogrepl / wal2json / Debezium)
Exact, commit-ordered, skew-free, but `wal_level=logical` + REPLICATION priv,
**replication slots pin WAL (disk-fill foot-gun)**, slot ops. Large. Overkill
for in-app consumers on one DB. Rejected.

### Cross-cutting: sequence skew (affects A's backstop and B)
`seq` is assigned at insert-time but rows commit out of order, so a naive `seq >
lastSeen` reader between two commits can skip a lower seq that committed late.
Fixes: **(i) `xid8` column + `xid < pg_snapshot_xmin(pg_current_snapshot())`**,
advancing the watermark by the xmin horizon — provably exact, costs a migration
(PG 13+; trivial on a small DB, full-table rewrite on a large one). **(ii)
overlap window** (`seq > max - N` / re-scan a few seconds) + idempotent
re-snapshot — no migration, correct as long as txns commit within the window
(rela writes are short, so the window is generous). Only affects the *backstop*,
not the payload happy-path; consumers re-snapshot so apply is idempotent for
free.

## Recommendation

**Option A — LISTEN/NOTIFY with the changed `kind:op:id` IN the payload, backed
by a `seq` catch-up for recoverability.**

- The notification payload carries the identity fields, so a live notification
reconstructs the exact `store.Event` an in-process write would emit — targeted,
not a coarse "something changed." This is the granularity `LastModified()` alone
can't give and handles deletes cleanly.
- It is **recoverable, not best-effort**: the `seq` catch-up on reconnect + a
slow safety ticker recover anything missed while a process was disconnected.
NOTIFY is the fast path; the watermark catch-up is the correctness backbone.
- **Skew is handled on the backstop only**, and we start with the **overlap
window + idempotent re-snapshot (ii)** — no migration, proportionate given
rela's short transactions and re-snapshotting consumers. The **`xid8` + xmin
horizon (i)** is documented as the exact upgrade if a strictly-ordered /
exactly-once consumer (audit stream, external sync) ever appears.

Accepted tradeoffs: one dedicated LISTEN connection per process; reconnect
logic; the connection seam widens to hand pgstore a listening connection;
overlap-window skew handling is a (well-sized) heuristic on the recovery path.

**Scope note for planning:** search needs nothing (always-live). The real
consumer is cross-process live-reload (data-entry SSE) and possibly MCP — and
pgstore isn't wired into those today. Planning must decide whether this ticket
also wires the consumer(s) or lands the feed + watcher plumbing first.
