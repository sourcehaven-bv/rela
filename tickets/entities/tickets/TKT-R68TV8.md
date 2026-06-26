---
id: TKT-R68TV8
type: ticket
title: 'Finish dataentry.App decomposition: command/sync handlers + write nucleus (M5 follow-up)'
kind: refactor
priority: medium
effort: m
status: backlog
---

Follow-up to TKT-N26KLB. That ticket drove `dataentry.App` from 227 methods down
to ~166 and landed the structural payoff — the ACL-bounded `visibleReader` seam
that closes the #1010 read-ACL bug class — but stopped short of the original
`<40` end goal. We chose to ship that progress rather than block the plimsoll
ratchet on the whole refactor. This ticket tracks the remainder.

## Remaining steps (from the M5 plan)

- **M5.2** — extract command + sync handlers off `App`.
- **M5.4** — carve the write nucleus + entity/relation/attachment write handlers
  behind one shared `writeMu`; drive `App` under the 40-method line and delete
  the `//plimsoll:max-methods` directive in `internal/dataentry/app.go`.

## Invariants (unchanged from M5)

- Read handlers take the ACL-bounded `visibleReader` only — never `store.Store`.
- `writeMu` stays a single shared instance across all write handlers (race
  detector guards).

## Related finding

The read-path audit also surfaced an ungated nav-badge count leak
(`enrichNavEntry`, same #1010 read-ACL class) — tracked separately as GitHub
issue #1043. It should fall out naturally when the decomposition reaches the nav
handler.

## Done when

`App` is under the 40 total-method load line, the grandfathering directive in
`app.go` is gone, and plimsoll passes on `dataentry` with no override.
