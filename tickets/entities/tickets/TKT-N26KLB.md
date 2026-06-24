---
id: TKT-N26KLB
type: ticket
title: Decompose dataentry.App god object + structural ACL-bounded read API (M5)
kind: refactor
priority: medium
effort: l
status: in-progress
---

Sub-arc of TKT-N0IKN9 focused on `dataentry.App` — the highest-value god object
(227 methods, 13 exported) and the one tied to a real bug class.

## Why App is the priority

The read-ACL gate (`readGate`, `internal/dataentry/readgate.go`) exists but is
pulled from context and applied **manually** in each handler, next to a raw
`a.store` a handler can read unfiltered and forget to gate.
`dataentry/CLAUDE.md` states the intent as policy — *"reads → readGate, writes →
entitymanager"* — but nothing enforces it. That "gate by convention" is the
#1010 bug class.

## End goal

`App` under the 40 total-method line; read handlers take an **ACL-bounded
`visibleReader`** (composes `store.Store` + `readGate`, every read pre-gated),
never raw `store.Store`. Write handlers share one `writeMu` nucleus.

## Milestone steps (each a stacked PR)

- **M5.0** — delete dead `handlers_api.go` legacy `/api/` surface (13 test-only methods + 5 helpers + 9 `API*` types, verified unrouted; keep live settings/palette). 227→~209.
- **M5.0b** — introduce `visibleReader`; migrate in-place read handlers onto it (behavior-preserving, ACL tests pin wire shape).
- **M5.1** — extract read-only handler groups (analyze/views/theme) taking the read bundle.
- **M5.2** — extract command + sync handlers.
- **M5.3** — extract the affordances/ACL seam (translateVerb stays one constructor).
- **M5.4** — carve the write nucleus + entity/relation/attachment write handlers (shared writeMu). →<40, delete directive.

## Invariants

- Read handlers take the ACL-bounded reader only — never `store.Store`.
- `writeMu` stays a single shared instance across all write handlers (race detector guards).
