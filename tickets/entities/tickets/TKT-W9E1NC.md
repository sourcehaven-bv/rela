---
id: TKT-W9E1NC
type: ticket
title: 'Extract metaIndex off fsstore.FSStore: make the map+order pairing a single mutation so the comparator-mismatch bug class is unrepresentable'
kind: refactor
priority: medium
effort: l
tags:
    - tech-debt
status: ready
---

Sub-ticket of [[TKT-N0IKN9]], `fsstore.FSStore` arc. The highest-value
extraction in the arc: it removes a **class of bug**, not just methods.

## The problem it solves

`entities`/`entityOrder` and `relations`/`relationOrder` always mutate in the
same statement pair — 9 sites for `entityOrder`, 10 for `relationOrder`, across
`entity.go`, `index.go`, `relation.go`, `watcher.go`, `attachment.go`. Nothing
enforces the pairing today. `fsstore.go:452-472` documents a real past bug:
`SortedRemoveFunc` panicking because construction used a different comparator
than mutation. `propCache` co-mutates with `entities` at the same sites. Making
each pair ONE method on a type that owns all five fields makes the invariant
unbreakable.

## Shape (`internal/store/fsstore/metaindex.go`)

```go
type metaIndex struct {
    entities      map[string]entityMeta
    entityOrder   []string
    relations     map[string]relationMeta
    relationOrder []string
    propCache     map[string]map[string]int
}
func (x *metaIndex) putEntity(m entityMeta)                  // map + SortedInsertFunc, one comparator
func (x *metaIndex) removeEntity(key string) (entityMeta, bool)
func (x *metaIndex) putRelation(m relationMeta)
func (x *metaIndex) removeRelation(key string) bool
func (x *metaIndex) stateFamily(id string) ([]entityMeta, []relationMeta) // was FSStore.stateFamily (entity.go:391)
func (x *metaIndex) familySize(id string) int                            // was entity.go:612
func (x *metaIndex) idTaken(id, except string) bool
func (x *metaIndex) countProp(e *entity.Entity, delta int)               // add/removeEntityFromCache
```

Plain data structure, **no mutex** — `mu` stays on FSStore (arc-wide rule on the
attachmentStore ticket). Net method delta on FSStore is modest (~−6) but ~20
mutation sites collapse into the type.

## Consider in the same PR

`memstore.MemStore` holds the same five fields (memstore.go:73-88 are a strict
subset of fsstore's) and pairs them at its own sites. If `metaIndex` ends up
storage-agnostic, hoist it to `storeutil` and let memstore adopt it in a
follow-up — do not widen this PR to two backends.

## Do NOT

- Add a lock to the type.
- Shadow promoted methods: `txStore` (tx.go:132) embeds `*FSStore`; extracted
types must be reachable as plain fields.
- Touch `tx.go`, `emit`/observer notification ordering, or `notifyRenamed`
single-event semantics (do-not-touch list from TKT-Y683LJ still applies).

## Done when

`storetest.RunAll` + full fuzz corpus (`FuzzRenameKeyCollapse` guards this
directly), `go test -race ./internal/store/...`, ratchet
`//plimsoll:max-methods` (fsstore.go:159).
