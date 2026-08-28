---
id: IDEA-WIJ2H1
type: idea
title: Pluggable job queue with swappable backends (memory / postgres / redis)
description: A general job-queue seam in rela with backend implementations (in-memory, postgres, redis), so background work gets real durability, visibility and retry semantics instead of each subsystem hand-rolling a buffer.
category: architecture
inspiration: Mail outbox (TKT-332QZY) needs a delivery buffer; best-effort was accepted there deliberately, but the general need recurs.
effort: large
value: valuable
status: captured
---

The mail outbox in TKT-332QZY is deliberately **best-effort**: an in-process
buffer with retry, no durability guarantee across a crash or restart. That is
the right trade for shipping mail, but it is the second time rela has wanted a
queue (the pgstore version-capture sweep is a debounced reconciliation loop
solving an adjacent problem), and it will not be the last.

**The shape.** A `Queue` seam with the same posture as `store.Store`: a small
interface, backend implementations chosen at wiring, and one conformance suite
every backend must pass (the `internal/store/storetest` pattern).

- **memory** — the default; matches today's best-effort behaviour, no new dependency.
- **postgres** — durable, and rela already has the machinery: a `jobs` table in the
store's schema (so schema-per-tenant scopes it for free), `SELECT ... FOR UPDATE
SKIP LOCKED` for multi-consumer dispatch, and `LISTEN/NOTIFY` already wired for
the change feed (TKT-WZYWM9) to wake workers without polling.
- **redis** — for deployments that already run it and want queue traffic off the
primary database.

**What it would buy beyond mail:** durable retry across restarts, multi-process
dispatch without duplicate work, dead-letter handling for permanently failing
jobs, and operator visibility (depth, age, failures) — none of which a
per-subsystem buffer provides.

**Sequencing.** Deliberately *after* mail, not before. Mail proves the consumer
shape against a real workload; designing the queue first would be speculative.
When this is picked up, the mail outbox becomes its first consumer and its
best-effort caveat is retired.

**Cross-cutting note.** rela already refuses to stack abstractions over
`store.Store` (no repo/tx layers, DEC-8UIL0). A queue is not that — it is a peer
subsystem with its own backends, not a wrapper — but the design should say so
explicitly, or it will read like the thing the house rules forbid.
