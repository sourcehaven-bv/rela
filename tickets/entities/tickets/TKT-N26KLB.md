---
id: TKT-N26KLB
type: ticket
title: Decompose dataentry.App god object + structural ACL-bounded read API (M5)
kind: refactor
priority: medium
effort: l
status: done
---

Sub-arc of TKT-N0IKN9 focused on `dataentry.App` — the highest-value god object
(started at 227 methods, 13 exported) and the one tied to a real bug class.

## Why App is the priority

The read-ACL gate (`readGate`, `internal/dataentry/readgate.go`) exists but is
pulled from context and applied **manually** in each handler, next to a raw
`a.store` a handler can read unfiltered and forget to gate.
`dataentry/CLAUDE.md` states the intent as policy — *"reads → readGate, writes →
entitymanager"* — but nothing enforces it. That "gate by convention" is the
#1010 bug class.

## What landed (227 → 166 methods)

- **M5.0** — deleted the dead `handlers_api.go` legacy `/api/` surface
  (test-only methods, helpers, `API*` types; live settings/palette kept).
- **M5.0b** — introduced the ACL-bounded **`visibleReader`** seam
  (`internal/dataentry/visiblereader.go`) and migrated read handlers onto it.
  This is the structural payoff: a handler holding `visibleReader` instead of a
  raw `store.Store` *cannot* skip the read gate — it closes the #1010 bug class.
- **M5.1** — extracted the read-only handler groups: `analyze.go`, `views.go`,
  `default_view.go`, `handlers_theme*.go`.
- **M5.3** — extracted the affordances/ACL seam (`affordances*.go`,
  `userstate.go`, `entityserializer.go`); `translateVerb` stays one constructor.

## Deliberately deferred (tracked in TKT-R68TV8)

We are shipping incremental progress rather than blocking the plimsoll ratchet
on the full refactor — not all tech debt has to clear in one go.

- **M5.2** — extract command + sync handlers off `App`.
- **M5.4** — carve the write nucleus + write handlers behind one `writeMu`,
  drive `App` under the 40-method line, delete the `//plimsoll:max-methods`
  directive in `app.go`.

`App` is grandfathered at `//plimsoll:max-methods` pinned to its current count
(so it can't grow); the follow-up ratchets it the rest of the way to <40.

## Invariants (carried into the follow-up)

- Read handlers take the ACL-bounded reader only — never `store.Store`.
- `writeMu` stays a single shared instance across all write handlers (race
  detector guards).
