---
id: TKT-ODHV2D
type: ticket
title: Restore incoming/relation columns in v1 list handler (regressed in Vue SPA migration)
kind: refactor
priority: medium
effort: s
tags:
    - regression
status: ready
---

## Problem

A list column `- relation: <rel>, direction: incoming` renders empty even when
incoming edges exist. More broadly, relation columns on lists are resolved
**client-side** from the wire `relations` map, which only carries **outgoing**
target IDs.

## Why this is a regression, not a new gap

`FEAT-tr9f` / `FEAT-0c9l` shipped `direction: incoming` for list columns by
adding `Direction` to `ListColumn` and making `resolveRelationColumnValues()`
incoming-aware. That code still exists (`dataentryconfig/config.go:208`,
`dataentry/helpers.go:679`). But the feature was wired into the **old
server-rendered `handleList`**, which was **removed in the Vue SPA migration
`ebbcb5b6` (#230)**. The current list path — `handleV1ListEntities`
(`api_v1.go:291`) — serializes rows via `forWireRelated(...,
a.reader.outgoingRelations(...))` (`api_v1.go:321`) and **never calls
`resolveRelationColumnValues`**. Only `sections.go:257` (table sections) still
does. So incoming list columns silently regressed.

## Scope

IN: incoming relation columns render correctly in `handleV1ListEntities` + SPA
`EntityList.vue`. OUT: filter controls (separate ticket), kanban cards (separate
ticket).

## Recommended approach (verified against develop tip)

Do **not** add a new `RelationColumns map[string][]string` title map (the
incoming spec's suggestion). That would fork the rendering path. The existing
mechanism already does client-side ID→title resolution:

- Wire: `v1.Entity.Relations[relType] = [outgoing target IDs]` (`entityserializer.go:52`).
- SPA: `?include=*` fetches peers; `getFormattedCellValue` (`EntityList.vue:518`) maps IDs → titles via the `included` map.

Key fact: `resolveV1Includes` **already** collects incoming source entities into
`included` when `include == "*"` (`api_v1.go:1781`). The only missing link is
that the serialized `relations` map records outgoing `edge.To` only. Fix:

1. For list rows, populate incoming edges into the serialized `relations` map keyed by the inverse relation name (mirroring `handleV1EntityRelations:1016`), so an incoming column has a lookup key without colliding with the same relation used outgoing.
2. SPA: `EntityList.vue` — `hasRelationColumns` and `getFormattedCellValue` become direction-aware (look up the incoming key when `column.direction === 'incoming'`); the `?include=*` fetch already covers sources.

## Acceptance criteria

- `PERS-JV --verantwoordelijk_voor--> TASK-VKZ2`; taak list column `{relation: verantwoordelijk_voor, direction: incoming}` → cell shows "Jeroen Vloothuis".
- Same column with `direction: outgoing` → empty cell.
- Two persoons pointing at one taak → cell shows both, comma-separated.
- Existing outgoing relation columns render byte-identically (no regression).

## Test plan

- Go handler test: list response for a taak carries the incoming source under the expected key; outgoing unaffected.
- SPA unit test on `getFormattedCellValue` for incoming direction.
- Regression assertion that outgoing columns are unchanged.
