---
id: TKT-PVMWOZG
type: ticket
title: 'pgstore: measure keeping the pooled connection on context cancel (pgx CancelRequestContextWatcherHandler)'
kind: chore
priority: low
effort: s
status: backlog
---

## Description

When a request context is canceled during a query, pgx's default context watcher
(`DeadlineContextWatcherHandler`) sends a cancel request and then closes the
pooled connection. The next request opens a new backend session (TLS + auth in
production, plus a cold prepared-statement cache).

Measured during TKT-T2I7P2 (local seed, scale 1): one typing run with 12 aborted
`/_search` requests opened 11 new backend sessions.

pgx offers `CancelRequestContextWatcherHandler`, which keeps the connection and
waits for the canceled statement to end.

Measure first, then decide:

1. Cost of a reconnect on atlas-like settings (TLS, scram) vs. the wait
for a canceled statement to drain.
2. Behaviour under a stuck query (DeadlineDelay as a backstop).
3. Effect on pool exhaustion when many requests cancel at once.

Out of scope until measured: any change to `pgstore.Open`.
