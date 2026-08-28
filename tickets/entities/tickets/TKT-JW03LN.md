---
id: TKT-JW03LN
type: ticket
title: 'Authorize incoming-side renumber writes; pin OpRename-on-relations unreachability'
kind: enhancement
status: backlog
priority: medium
effort: s
---

## Split from TKT-XZEY at design review (DR-5, DR-25)

Two small, related hardening items on relation writes. Depends on
**TKT-K2VN9D** for the per-relation `update:` permission.

## 1. Incoming-side renumber writes unauthorized relations (DR-5)

`runRenumberAfterUpdate` (`manager_order.go:210-225`) fires two queries:

```go
if touchedOut { q := store.RelationQuery{From: from, Type: relType} ... }  // :217
if touchedIn  { q := store.RelationQuery{To: to,   Type: relType} ... }    // :222
```

The **incoming** query selects every relation of that type pointing at `to` —
from **arbitrary other source entities of arbitrary other types**.
`maybeRenumberSide` then writes each via `Store.UpdateRelation` **directly**,
deliberately bypassing authz (`:186-202`, to avoid recursing through
`UpdateRelation`).

TKT-XZEY originally dismissed this as "rides the user's already-authorized
update. Fine." That is true for the OUTGOING side and **false for the
incoming side**.

After TKT-K2VN9D a principal can hold `relation_grants: {R: {update: reorder-R}}`
and **no entity-type update grant at all**, then one authorized order-property
PATCH on their own `X --R--> shared` edge triggers renumber writes to
`A --R--> shared`, `B --R--> shared`, … where `A`/`B` are entities of types the
principal cannot touch by any other route.

Bounded to the order property (existing props are copied, `:194-196`), so it is
value manipulation rather than arbitrary injection — but relation order is
semantically meaningful (it drives SPA rendering and `sort:` views), and the
audit rows attribute the writes to a principal who demonstrably lacks authority.

**Fix.** Authorize the incoming-side renumber plan entries before applying.
`manager_order.go:180-182` is already two-phase (plan, then apply), so this is
a loop over `plan` before the apply loop. Prefer this over restricting the
query, which would change renumber's densification semantics.

## 2. Pin the OpRename-on-relations unreachability (DR-25)

`grantsVerb` routes `OpRename` through the `Update` list
(`internal/acl/policy.go:310`). For relations that routing is currently
**unreachable**: `Manager.RenameEntity` (`manager.go:947`) makes exactly one
ACL call, with an `EntitySubject` (`:964-967`), and the re-key is delegated to
`Store.RenameEntity` which rewrites every incident endpoint in one backend
operation. `RelationSubject`'s godoc confirms the intended set: "used for:
Create / Delete of a relation" (`subject.go:38`).

That is a correct decision, but it is currently only true *by accident of no
caller*. Once a per-relation `update:` permission exists, a future
`RelationSubject` + `OpRename` caller would **silently inherit `Update`
semantics** from the new block.

**Fix.** Add a test asserting no non-test construction site pairs `OpRename`
with a `RelationSubject` (or, if a caller is ever wanted, that it is a
deliberate, separately-reviewed decision). Cheap insurance; same shape as the
existing `lint_test` construction-site invariants.

## Acceptance criteria

1. Incoming-side renumber denies (or omits) writes to relations the principal
   is not authorized to update; outgoing-side behaviour unchanged.
2. Densification semantics for the authorized subset are unchanged.
3. A test pins `OpRename` + `RelationSubject` unreachability.

## Files

`internal/entitymanager/manager_order.go` (+ test),
`internal/acl/` or `internal/entitymanager/` lint-style test.
