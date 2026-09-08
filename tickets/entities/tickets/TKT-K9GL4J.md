---
id: TKT-K9GL4J
type: ticket
title: 'Extract liveFeed off dataentry.App: one owner for the reload/SSE lifecycle with Start/Stop (~69 → ~60)'
kind: refactor
priority: medium
effort: m
tags:
    - tech-debt
status: backlog
---

Sub-ticket of [[TKT-R68TV8]] (the `dataentry.App` arc under [[TKT-N0IKN9]]).

## Why this is an abstraction

This is the one App cluster with a lifecycle and goroutines, and its ownership
is currently muddled: `startStoreEventBridge` (watcher.go:209) spawns `go
a.pumpStoreEvents(events)` and parks the cancel in an App field; `StopWatching`
(app.go:439) releases it. "Own the two subscriptions and the goroutine that
drains them" becomes a single object with `Start`/`Stop`, so it cannot be
half-stopped. It also owns `rebuildState` — the atomic Schema publish — which
pairs correctly with subscribe: they are the two halves of "react to a change",
and this type becomes the SOLE writer of the published snapshot.

## What moves (8 methods, `internal/dataentry/watcher.go`)

`StartWatching` (:145), `startStoreEventBridge` (:209), `pumpStoreEvents`
(:217), `StartGitFetch` (:242), `rebuildState` (:280), `handleSSE` (:329),
`runSSELoop` (:385), `freshReadGate` (:471). `entityTypeVisible` (:500) has an
unused receiver — package function. `StopWatching` stays on App as a delegation
(public API). `noCacheMiddleware` stays on App (reads `principalHeader`, applied
in `NewRouter`).

Fields leaving App: `cfgLoader`, `broker`, `gitOps`, `stopConfigWatch`,
`stopStoreWatch`.

## Shape

```go
type liveFeed struct {
    schema    schemaProvider
    cfgLoader config.Loader
    palette   *paletteService
    events    storeSubscriber  // consumer-side: Subscribe only
    watcher   storeWatcher     // consumer-side: StartWatching only
    gitOps    *git.Ops
    broker    *eventBroker
    acl       func() acl.ACL   // closure: tests rebind app.acl (183 sites)

    mu       sync.Mutex
    stopCfg  func()
    stopFeed func()
}

// storeSubscriber is the change-feed capability liveFeed needs — declared
// at the consumer. One method of store.Store's ~30.
type storeSubscriber interface {
    Subscribe(buf int) (<-chan store.Event, func())
}

func (f *liveFeed) Start() error   // was App.StartWatching
func (f *liveFeed) Stop()          // releases exactly what Start acquired
```

## Invariants

- The SSE broadcast carries NO audit detail and NO entity id
(`startStoreEventBridge`'s audit-isolation invariant, pinned by
`TestSSE_DoesNotFlowAuditEvents`). Move the comment with the code.
- `Stop` is idempotent and releases exactly what `Start` acquired; add a test
for double-Stop and Stop-before-Start.
- Tests reassign `app.broker` (54 sites) and `app.acl` (183 sites) after
construction: hold those as closures or via a rebind hook, never captured
values. Check for a shared test builder before hand-editing files.
- Race detector: `go test -race ./internal/dataentry/...` is the gate.

Ratchet `//plimsoll:max-methods` (app.go:172) to the new count.
