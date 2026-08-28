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

> **Update (2026-07-26): views cluster carved — [[TKT-I37338]]. App at 90 methods, directive 90. The dead server-rendered nav path was deleted, closing GitHub #1043 (ungated nav-badge counts). Remaining toward <40: read handlers (list/get/relations/search/analyze/includes on api_v1.go, the largest cluster), config/schema/docs/templates endpoints, and the misc app.go surface.**

**Epic / parent ticket** for the remainder of the `dataentry.App` decomposition.
Each shippable step is its own sub-ticket (moved to `done` with its PR); this
parent stays in `backlog` until the whole arc lands — App under the 40-method
line with the plimsoll directive deleted. (Per-PR sub-tickets satisfy the
done-before-merge gate honestly: a multi-PR epic can't be `done` while work
remains, and `backlog` is the allowed "not-touched-by-this-PR" parent state.)

Follow-up to the earlier decomposition that drove `dataentry.App` from 227
methods down to ~166 and landed the structural payoff — the ACL-bounded
`visibleReader` seam that closes the #1010 read-ACL bug class — but stopped
short of the original `<40` end goal. The AppState state-decomposition
prerequisite ([[TKT-XSWFXQ]]) is also done, so the snapshot/`writeMu` publish
coupling is gone and the handler clusters can move off `App` one at a time.

## Sub-tickets

- **M5.2 — extract command + sync handlers off `App`.**
  - [x] [[TKT-VG9P1]] — sync route cluster → `syncHandler` (170 → 154). PR #1134.
  - [x] [[TKT-KUFLD]] — command route cluster → `commandHandler` (154 → 143).
PR #1138.
  - [x] [[TKT-QTPUA]] — attachment cluster → `attachmentHandler` + package
functions (143 → 131). PR #1149.
- **M5.4 — write nucleus.** Carve the entity/relation/attachment write handlers
behind one shared `writeMu`; drive `App` under 40 and delete the
`//plimsoll:max-methods` directive in `internal/dataentry/app.go`.
  - [x] [[TKT-HKY8RJ]] — write nucleus (entity/relation CRUD + dry-run, clone,
conflict-resolve, relations reconciler; 18 methods) → `writeHandler` (131 →
114). PR #1191.
  - [x] [[TKT-6NDSH9]] — Lua action handler + webhook dispatch → `writeHandler`
(115 → 114 after develop drift), completing the write surface.
  - **Open question — settled.** Researched in [[RES-Z1SJ5]], decided in
[[DEC-8UIL0]]: write serialization becomes a `Tx` contract on `store.Store` (fs
= mutex, postgres = native transactions + advisory lock), implemented as its
**own follow-up arc** that deletes `writeMu` outright. M5.4 itself proceeds
conservatively: the mutex moves with the write-nucleus struct, semantics
untouched — refactors don't change concurrency behavior.
- **M5.5 — views cluster.**
  - [x] [[TKT-I37338]] — view traversal + section builders + /_views,
/_sidepanel, /_sidebar → `viewsHandler`; 3 helpers package-leveled; dead ungated
nav path deleted (114 → 90), closing GitHub #1043.

## Invariants (unchanged)

- Read handlers take the ACL-bounded `visibleReader` only — never `store.Store`.
- `writeMu` stays a single shared instance across all write handlers (race
detector guards). The extracted handlers hold a *pointer* to `App`'s `writeMu`,
preserving this. (Holds until the [[DEC-8UIL0]] arc replaces the mutex with
store `Tx`.)

## Related finding

The read-path audit also surfaced an ungated nav-badge count leak
(`enrichNavEntry`, same #1010 read-ACL class) — tracked as GitHub issue #1043.
**Closed by [[TKT-I37338]]**: the leaky path was production-dead (SPA-era);
deleting it removes the leak, and the live gated sidebar count path is pinned by
`TestACLSidebar_CountsMatchList`.

## Done when

`App` is under the 40 total-method load line, the grandfathering directive in
`app.go` is gone, and plimsoll passes on `dataentry` with no override.
