---
id: TKT-N3X235N
type: ticket
title: 'tenant mode: background work and webhook dedup across eviction; faster shutdown'
kind: enhancement
priority: medium
effort: m
status: backlog
---

## Description

Follow-up from the TKT-D6MF3SW code review. A tenant's per-process state ends
when the registry evicts it, and tenant-mode shutdown does not end live streams
first.

- Background work stops while a tenant is not open: its durable job queue,
mail outbox, soft-delete GC and version sweep run only while the tenant is
resident. A mail enqueued just before eviction waits until the tenant is next
used.
- The IdP webhook replay set lives on each tenant's App, so eviction resets
it and a captured webhook can be replayed within its JWT lifetime. The action
refetches authoritative data, which limits the impact.
- Tenant-mode shutdown does not end tenant SSE streams before
`srv.Shutdown`, so with an open tab it waits the full grace and exits 1.
Residents then close one at a time.
- A tenant evicted while a stream holds it stays open until the stream
ends. The hard ceiling (`max_open`) bounds this; a reconnect-later hint to
pinned streams would free slots sooner.

## Approach

- Run tenant background jobs from a tenant-independent runner, or keep
residents with queued jobs open until drained.
- Keep the webhook seen-set in the tenant's `state.KV` with a TTL.
- Register a shutdown hook that retires every resident's streams, and close
residents in parallel.
