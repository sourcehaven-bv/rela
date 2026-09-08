---
id: TKT-R68TV8
type: ticket
title: 'Finish dataentry.App decomposition: command/sync handlers + write nucleus (M5 follow-up)'
kind: refactor
priority: medium
effort: m
status: backlog
---

> **Sweep note (2026-07-20): M5.2 sub-tickets confirmed landed (sync_handler.go, command_handler.go, attachment_handler.go all exist). Epic remains open: App still has 132 receiver methods (goal <40), //plimsoll:max-methods=131 directive still present in app.go, and the M5.4 write nucleus is not carved (writeMu still a field on App, taken directly in api_v1.go write handlers).**

> **Update (2026-07-25): M5.4 write nucleus carved — [[TKT-HKY8RJ]] / PR #1191 merged, then [[TKT-6NDSH9]] moved the Lua action + webhook dispatch in as well. App at 114 methods, directive 114. The write surface is COMPLETE on writeHandler: no App method takes writeMu directly anymore. writeMu itself remains App-owned and pointer-shared; deleting it outright stays the [[DEC-8UIL0]] Tx arc, which now has a single obvious seam.**

> **Update (2026-07-26): views cluster carved — [[TKT-I37338]]. App at 90 methods, directive 90. The dead server-rendered nav path was deleted, closing GitHub #1043 (ungated nav-badge counts).**

> **Update (2026-09-04): App at 87 (the worlds feature added one). Remaining
> arc re-planned as four extractions, each a real type with a closed field set
> and an explicit-deps constructor (no more `newX(app *App, …)` — that shape,
> used by viewsHandler/appearanceHandler/queryService/exportHandler, is a
> service-locator seam and is not repeated). See M6 below.**

**Epic / parent ticket** for the remainder of the `dataentry.App` decomposition.
Each shippable step is its own sub-ticket (moved to `done` with its PR); this
parent stays in `backlog` until the whole arc lands — App under the 40-method
line with the plimsoll directive deleted.

Follow-up to the earlier decomposition that drove `dataentry.App` from 227
methods down to ~166 and landed the structural payoff — the ACL-bounded
`visibleReader` seam that closes the #1010 read-ACL bug class — but stopped
short of the original `<40` end goal.

## Sub-tickets

- **M5.2 — extract command + sync handlers off `App`.**
  - [x] [[TKT-VG9P1]] — sync route cluster → `syncHandler` (170 → 154). PR #1134.
  - [x] [[TKT-KUFLD]] — command route cluster → `commandHandler` (154 → 143). PR #1138.
  - [x] [[TKT-QTPUA]] — attachment cluster → `attachmentHandler` (143 → 131). PR #1149.
- **M5.4 — write nucleus.**
  - [x] [[TKT-HKY8RJ]] — write nucleus → `writeHandler` (131 → 114). PR #1191.
  - [x] [[TKT-6NDSH9]] — Lua action handler + webhook dispatch → `writeHandler`.
  - Settled in [[RES-Z1SJ5]] / [[DEC-8UIL0]]: write serialization becomes a
`Tx` contract on `store.Store`, its own follow-up arc.
- **M5.5 — views cluster.**
  - [x] [[TKT-I37338]] — views/sidepanel/sidebar → `viewsHandler` (114 → 90).
- **M6 — the remaining four surfaces (87 → ~45), safest first:**
  - [ ] [[TKT-XDJTDC]] — `configAPI`: principal-independent metadata surface
(schema/config/openapi/templates/apps). No store, no reader, no ACL field. Also
fixes the stale worldneighbors.go:85 prose. (−7)
  - [ ] [[TKT-NLX424]] — `nextActionAPI`: suggestion adapter; mutex-guards the
late-set `userState`/`matchers`. (−11)
  - [ ] [[TKT-K9GL4J]] — `liveFeed`: reload + store-event bridge + SSE as one
Start/Stop lifecycle; sole writer of the published Schema. (−9)
  - [ ] [[TKT-CU105Y]] — `entityAPI`: v1 entity read surface; exclusively
owns `reader`/`visibleReader`/`serializer`/`worldNeighbors`; consumer-side
`entityWriteRoutes` interface toward writeHandler. (−15)
- **M7 (decide after M6)** — `listPipeline` for `scopedSortedEntities` +
`applyRelationFilters` + `matchRelationFilter` + `resolveScope` +
`handleV1EntityPosition` (5 methods, 5 consumers). Deliberately NOT in M6:
absorbing it into `entityAPI` would make gantt/export/next-action depend on the
read surface — a hub-and-spoke worse than today.

## Invariants (unchanged)

- Read handlers take the ACL-bounded `visibleReader` only — never `store.Store`.
- Late-set App fields (`worlds`, `worldNeighbors`, `userState`,
`nextActionMatchers`, `caldavAliases`, `webhook`) are set by public setters
AFTER `NewApp`; extracted types hold them as closures or behind a mutex, never
captured by value (the `viewsHandler.faceEdges` lesson).
- Tests reassign App fields post-construction (`app.acl` 183×, `app.broker`
54×, `app.fieldResolver` 52×): closures over App, not copies.
- One `State()` snapshot per handler.
- Free-win package functions (unused receivers): `renderHelpContent`,
`gatherRelations`, `toDocumentRenderConfig`, `entityTypeVisible`,
`applyNextActionFeedback`, `gateReadOrNotFound`, `currentEdgesByPeer`,
`denyAffordance` — each M6 ticket takes the ones in its cluster.

## Done when

`App` is under the 40 total-method load line, the grandfathering directive in
`app.go` is gone, and plimsoll passes on `dataentry` with no override.
