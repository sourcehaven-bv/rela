---
id: TKT-ONFXVS
type: ticket
title: 'storeutil.ObserverSet: one home for the FaceObserver/bare-id dispatch contract currently copied into four backends'
kind: refactor
priority: low
effort: m
tags:
    - tech-debt
    - needs-design
status: backlog
---

Sub-ticket of [[TKT-N0IKN9]], `fsstore.FSStore` arc — cross-backend, so it lands
AFTER the fsstore-local extractions and needs a `/design-review` before
starting. Two survey agents disagreed on whether this should be done; the
disagreement is recorded here so the reviewer can decide.

## The duplication

`notifyPut` / `notifyFaceDelete` / `notifyLastFaceDelete` / `notifyRenamed`
exist in all four backends: `fsstore/fsstore.go:389-430`,
`memstore/memstore.go:151-186`, `sqlitestore/observer.go:39-100`,
`pgstore/entity.go:861-915`. fs and mem are identical modulo receiver. The
mutual-exclusion rule between `FaceObserver` and bare-id observers is a subtle
correctness contract stated in four separate doc comments — one home means one
place to get it right.

pgstore turned `notifyFaceDelete` into a package function purely to stay under
the plimsoll line (pgstore/entity.go:872-875 says so). That is the method cap
being gamed, and this ticket lets it be undone.

## The case against (sqlitestore/observer.go:40-47)

pg and sqlite prepend a `txPending` deferral: an observer that fires inside a
transaction that later rolls back leaves the search index holding an entity the
store does not have, and `RunTxRollbackTests` does not catch it. A shared helper
would have to take that seam as a parameter, re-introducing per-backend
divergence inside the function whose purpose was to remove it. Four small
correct copies may beat one parameterised abstraction.

## Proposed shape, if it proceeds

```go
// storeutil/observers.go
type ObserverSet struct{ obs []store.EntityObserver }
func (o *ObserverSet) Add(x store.EntityObserver)              // nil-drop
func (o *ObserverSet) Put(e *entity.Entity)
func (o *ObserverSet) FaceDelete(id string, f entity.Face)     // FaceObserver only
func (o *ObserverSet) LastFaceDelete(id string)                // bare-id only
func (o *ObserverSet) Renamed(oldID string, e *entity.Entity)
```

Zero value usable. For the tx-deferred backends the set is invoked from inside
the existing `txPending` hook, so the set itself knows nothing about
transactions — the deferral stays where it is, the dispatch body is shared.
Decide at design review whether that boundary is clean enough to be worth it.

## Explicitly out of scope

- A shared `SubscriberSet`/event hub. The four `emit` implementations are NOT
equivalent: fs/mem emit under `mu`, sqlite sends outside the lock
(events.go:61-77), pg/sqlite buffer through `txPending`. Flattening those
silently changes documented per-backend guarantees.
- A shared `txSerializer` for the 24 duplicated `tx.go` wrappers in fs/mem:
the duplication is structural (each wrapper names a distinct interface method);
erasing it needs generics over method sets or codegen, and would break the
compile-time `var _ store.Store = txStore{}` check. Document it as the known
floor instead.

arch-lint: all four backends may depend on `storeutil` (.go-arch-lint.yml
:1118-1165); `storeutil` may depend only on `store` (:1166) — confirm `entity`
is reachable or route via `store`'s aliases.
