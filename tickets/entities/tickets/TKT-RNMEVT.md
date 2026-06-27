---
id: TKT-RNMEVT
type: ticket
title: Single-event rename via EntityObserver.EntityRenamed
kind: enhancement
priority: medium
effort: s
status: done
---

## Description

Add `EntityRenamed(oldID string, renamed *entity.Entity)` to
`store.EntityObserver` and collapse the rename code path's existing
`EntityDelete(oldID)` + `EntityPut(renamed)` pair into one event, so observers
react to a rename as a single atomic callback instead of inferring it from a
delete/put sequence.

Prep work for the upcoming analysis-waivers ticket (TKT-WHHU), which wants to
react to renames without a `Store.GetEntity` round-trip.

## What

- **One callback per rename, not two.** Both `memstore` and `fsstore`
  previously fired `EntityDelete(oldID)` + `EntityPut(renamed)` on rename; now
  they fire only `EntityRenamed(oldID, renamed)`. This closes the half-renamed
  window where ID-keyed observers saw the old key disappear before the new one
  arrived.
- **Payload carries the renamed entity**, so content-driven observers don't
  need a store round-trip:
  - `bleveindex.Index.EntityRenamed` does delete+index in one Bleve batch.
  - `search.LinearSearch.EntityRenamed` swaps the map key under one critical
    section.
  - `pgstore.SearchBackend.EntityRenamed` is a no-op: `RenameEntity` rewrites
    the row id and recomputes `search_text` in the same transaction, so the
    search backend (which holds no derived state) has nothing to mirror —
    consistent with its no-op `EntityPut`/`EntityDelete`.

## Scope

In scope:
- `store.EntityObserver.EntityRenamed` interface method + single-event contract
  doc.
- `memstore`/`fsstore` rename path emitting the single event.
- `EntityRenamed` implementations on all search backends (bleve, linear,
  pgstore).
- Contract test in `internal/app/factory_test.go` pinning "rename = exactly one
  EntityRenamed, no put or delete".

Out of scope:
- `internal/rename`, which orchestrates rename as Create+Delete (not
  `store.RenameEntity`) for historical reasons — that path still emits the old
  put/delete pair. Switching it is the right end-state but a separate change.

## Acceptance criteria

- A rename through `store.RenameEntity` delivers exactly one `EntityRenamed`
  callback (no `EntityPut`/`EntityDelete`) carrying the entity under its new ID.
- All three search backends keep the index consistent across a rename (old ID
  gone, new ID findable) with no half-renamed window.
- The `postgres`-tagged build links and the conformance suite is unchanged.
