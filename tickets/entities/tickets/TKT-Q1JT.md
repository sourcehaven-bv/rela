---
id: TKT-Q1JT
type: ticket
title: Wire workspace search backend as fsstore Observer (drop Subscribe goroutine)
kind: refactor
priority: medium
effort: s
status: backlog
---

## Summary

After TKT-LCTG, workspace's search backend is kept in sync via `store.Subscribe`
→ buffered channel → goroutine → `EntityPut`. The canonical path used by
`internal/store/{memstore,fsstore}/conformance_test.go` is to pass the backend
directly via `fsstore.Config.Observers` (or equivalent for memstore), so the
store invokes observers synchronously under its write lock. That eliminates:

- The buffered-channel drop hazard (32-slot, silent loss under burst writes).
- The subscription goroutine + WaitGroup join cost in `Close()`.
- The race window between subscription cancel and `searchBackend.Close()`.

## In scope

- Replace `Workspace.startSearchSubscription` + `searchSubscriptionLoop` with passing `searchBackend` (a `store.EntityObserver`) into the store at construction.
- For fsstore: `app.FSFactory` grows a `WithObserver(store.EntityObserver)` knob so the workspace can hand its backend to `fsstore.Config.Observers`.
- For memstore in `NewForTest`: the memstore equivalent (it has the same observer hook).
- Drop `stopSearch`, `searchWG`, the goroutine, the buffered channel.

## Out of scope

- Changes to `search.Backend` or `bleveindex.Index`.
- MCP/CLI/dataentry migrations (TKT-KWAX/TKT-0SP1/TKT-9JEI).

## Depends on

- TKT-LCTG must land first. Without the backend-as-Observer shape, this is impossible.

## Why

Caught by cranky review on TKT-LCTG as significant finding #7/#13. The
Subscribe-relay pattern was preserved-as-is in TKT-LCTG to keep the diff a clean
lift, but the canonical pattern is the Observer wiring. This ticket completes
the migration.

## Risks

- `fsstore.Config.Observers` is currently set at construction; the workspace constructs the store before it constructs the search backend. Either: (a) build the backend first and pass it as a factory option (cleanest), (b) defer search-index population until both exist and observe-from-first-write. (a) is simpler.
- Observers are called synchronously under the store write lock. A slow EntityPut would back-pressure writes — but Bleve in-memory is fast and EntityPut is non-blocking, so this is not a real concern for the in-memory backend.
- TKT-KWAX/TKT-0SP1/TKT-9JEI may be in flight at the same time. Coordinate ordering so they don't conflict with the factory-option change.
