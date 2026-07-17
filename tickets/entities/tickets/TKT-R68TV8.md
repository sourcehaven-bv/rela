---
id: TKT-R68TV8
type: ticket
title: 'Finish dataentry.App decomposition: command/sync handlers + write nucleus (M5 follow-up)'
kind: refactor
priority: medium
effort: m
status: backlog
---

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
  - **Open question — settled.** Researched in [[RES-Z1SJ5]], decided in
[[DEC-8UIL0]]: write serialization becomes a `Tx` contract on `store.Store` (fs
= mutex, postgres = native transactions + advisory lock), implemented as its
**own follow-up arc** that deletes `writeMu` outright. M5.4 itself proceeds
conservatively: the mutex moves with the write-nucleus struct, semantics
untouched — refactors don't change concurrency behavior.

## Invariants (unchanged)

- Read handlers take the ACL-bounded `visibleReader` only — never `store.Store`.
- `writeMu` stays a single shared instance across all write handlers (race
detector guards). The extracted handlers hold a *pointer* to `App`'s `writeMu`,
preserving this. (Holds until the [[DEC-8UIL0]] arc replaces the mutex with
store `Tx`.)

## Related finding

The read-path audit also surfaced an ungated nav-badge count leak
(`enrichNavEntry`, same #1010 read-ACL class) — tracked separately as GitHub
issue #1043. It should fall out naturally when the decomposition reaches the nav
handler.

## Done when

`App` is under the 40 total-method load line, the grandfathering directive in
`app.go` is gone, and plimsoll passes on `dataentry` with no override.
