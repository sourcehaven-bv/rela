---
id: TKT-WZYWM9
type: ticket
title: Multi-writer support for pgstore (cross-process change feed)
kind: enhancement
priority: medium
effort: l
status: in-progress
---

## Summary

Make the PostgreSQL backend correct under **multiple concurrent writers** (e.g.
two `rela-server` processes, or `rela-server` + `rela mcp` + a `rela` CLI
invocation) against one database. Today `pgstore` is single-writer: its
`store.Watcher` is in-process only, so a second writer's changes are invisible
to the first process's derived state (search index, live-reload / SSE feed, MCP
change notifications), which silently drift from the database.

Follow-up to TKT-M8400, which deliberately scoped to a single rela-server
process owning the database. The schema was built for this: every
entities/relations/attachments row already carries `created_at`, `updated_at`,
and a global monotonic `seq` (from the `rela_seq` sequence), so a cross-process
change feed can reconcile from a watermark **without a schema migration**.

## Motivation

The original motivation for a PostgreSQL backend was multi-process / server
deployments and concurrent access. TKT-M8400 delivered the durable store and
in-DB search but punted on cross-process change propagation. Without it, running
more than one writer is a latent correctness bug (stale search index, missed
live-reload events) — not an error, just silent drift, which is the nastier
failure mode.

## Scope (refine in planning)

In scope:
- A cross-process change feed for `pgstore` so each process observes other
processes' committed writes and updates its derived state. Likely PostgreSQL
`LISTEN/NOTIFY` (notify with the changed key/seq on commit; each process
`LISTEN`s) PLUS catch-up: on (re)connect, replay everything with `seq >
last_seen_seq` so events missed while disconnected are recovered (NOTIFY has no
delivery guarantee and an 8KB payload limit).
- Reconnect / resubscribe logic for the dedicated listener connection
(connection drops must not permanently blind a process).
- Honor the existing `store.Watcher` contract (lossy buffered fan-out to
in-process subscribers) on top of the cross-process feed.
- Tests: multi-writer integration test (writer A's commit observed by writer
B's watcher), reconnect/catch-up test, race detector.

Out of scope (unless planning pulls them in):
- Changing the `store.Watcher` interface signature.
- HA / failover / replication topology.
- Applying the same to fsstore/memstore (single-process by nature).

## Acceptance criteria (draft — finalize in planning)

- With two pgstore-backed processes on one database, a write committed by
process A is delivered to a `Subscribe` channel in process B within a bounded
delay (assert in an integration test).
- A process that was disconnected from the feed and reconnects recovers the
writes it missed (catch-up from its last-seen `seq` watermark).
- Single-process behavior and the existing conformance suite are unchanged
(no regression for the single-writer deployment).
- No schema migration required beyond what TKT-M8400 shipped (verify the
`seq`/timestamp columns suffice).

## Notes / open questions for planning

- LISTEN/NOTIFY vs short-poll-on-seq vs logical replication — research the
trade-offs (`/research` candidate; the approach isn't obvious).
- Where the catch-up watermark lives (per-process in memory vs persisted).
- Ordering guarantees the feed must provide vs. what `store.Watcher` promises
(currently lossy + unordered).
- Interaction with the in-DB search backend (does it need its own catch-up, or
does it query live so it's always current?).
- Connection budget: a dedicated long-lived LISTEN connection per process.
