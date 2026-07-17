---
id: TKT-XSWFXQ
type: ticket
title: Decompose dataentry.App AppState into self-synchronized services + schema provider
kind: refactor
priority: medium
effort: m
status: ready
---

Continuation of the [[TKT-N26KLB]] `dataentry.App` decomposition arc (that
ticket is done; this is the next coherent unit — state, not handlers).

## Problem

`AppState` was an 8-field snapshot grab-bag published via `atomic.Pointer` on
`App`. It hoisted per-service mutable state (logo bytes, user palette, user
defaults) up to `App`, forcing every write of that state through `mutateState`
+ the App-wide `writeMu`. `rebuildState` hand-copied every field forward on
each reload. The mutex existed only because unrelated state shared one pointer.

## What landed (4 stacked PRs)

Peripheral-first: push each field's state down into a service that owns it
(self-synchronized behind its own `sync.RWMutex`), so the shared snapshot +
publish-mutex shrink to only genuinely co-derived reload state.

- **`logoStore`** — user-uploaded sidebar logo (bytes/ext/hash) + served cache.
Removed 3 `AppState` fields; 2 `mutateState` callers gone.
- **`paletteService`** — user palette override + resolved palette. The resolved
palette derives from `Cfg.Palette`, so the config→palette dependency is made
explicit as `Reresolve(cfgPalette)`. Removed 2 fields + the last `mutateState`
writes.
- **`settingsService`** — per-user default values. Removed the last field; with
zero callers left, **`mutateState` is deleted**.
- **`schemaProvider`** — the co-derived core `{Cfg, Meta, StyleMap, StyledTypes,
OpenAPIGen}`. `AppState`→`Schema`; the `atomic.Pointer` + reload derivation move
off `App` into the provider. `App.State()` delegates to `schema.Current()` so
all ~300 `a.State().Meta`/`Cfg` readers stay verbatim.

## Result

`AppState` (8-field grab-bag) → `Schema` (5-field co-derived core with a
dedicated provider). logo/palette/defaults each own their state behind their own
mutex. `mutateState` and the publish-side `writeMu` coupling are gone.

## Out of scope

- `writeMu`'s remaining Job-2 uses (entity-write serialization) — a separate
concern, belongs behind the store.
- The plimsoll `App` method-line is unchanged: this arc cuts state/fields, not
methods. The method-count ratchet is the handler-split follow-up
([[TKT-R68TV8]]).

## Delivery (stacked PRs)

- [x] `logoStore` (PR #1105)
- [x] `paletteService` (PR #1107)
- [x] `settingsService` + delete `mutateState` (PR #1109)
- [x] `schemaProvider` collapses `AppState` (PR #1110)
