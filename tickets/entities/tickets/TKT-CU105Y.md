---
id: TKT-CU105Y
type: ticket
title: 'Extract entityAPI off dataentry.App: the v1 entity read surface that exclusively owns the reader seams (~60 → ~45)'
kind: refactor
priority: medium
effort: l
tags:
    - tech-debt
    - security
status: ready
---

Sub-ticket of [[TKT-R68TV8]] (the `dataentry.App` arc under [[TKT-N0IKN9]]).
Largest App extraction; do it LAST in the arc so the other three have already
shrunk what it must reason about.

## Why this is an abstraction

It is the complete answer to "serve one entity or a page of them over v1,
correctly gated", with a real invariant to defend: **every entity body it emits
has passed the row-level read gate AND been resolved through the request's
world.** Today that invariant is spread across 13 App methods and is guarded
from the outside by `TestWorldCapableRoutesDoNotUseUngatedReader`
(entityreader.go doc) — i.e. by convention. After this PR the four seams that
decide it — `reader` (ungated, default-world), `visibleReader` (row-gated),
`worldNeighbors` (world-resolved links), `serializer` — are held on this type
and **nowhere else in the package**, so a new read path cannot acquire an
ungated handle by reaching through App. The compiler enforces what the guard
test currently asserts.

## What moves (13 methods + 2 helpers, `internal/dataentry/api_v1.go`)

`handleV1DynamicRoutes` (:176), `handleV1EntityCollection` (:261),
`handleV1SingleEntity` (:766), `handleV1ListEntities` (:648),
`handleV1GetEntity` (:814), `handleV1EntityRelations` (:964),
`handleV1EntityRelationType` (:1162), `handleV1GetRelationType` (:1182),
`handleV1RelationTarget` (:1247), `handleV1EntityAction` (:1266),
`resolveV1Includes` (:1775), `filterVisibleIncludes` (:1837),
`computeEntityETag` (:2131).

Two become package functions, which is SAFER than the status quo:
- `gateReadOrNotFound` (:801) uses zero App fields. As
`gateReadOrNotFound(w, r, typeName, entityID) bool` it cannot be handed a wider
handle. The uniform-404 rule at :795-798 ("any handler that 404s on the read
path MUST use this const") travels with it.
- `currentEdgesByPeer` (relations_direction.go:183) uses only `a.reader` —
takes an `entityReader` parameter.

## Shape

```go
type entityAPI struct {
    schema        func() *Schema
    reader        entityReader
    visibleReader visibleReader
    serializer    entitySerializer
    affordances   affordanceService
    // scoped is App.scopedSortedEntities as a closure — gantt, export and
    // next-action call the SAME function; a second implementation here would
    // be the ACL-ordering divergence its doc (api_v1.go:307-326) warns about.
    scoped     func(ctx context.Context, typeName string, q map[string][]string) ([]*entity.Entity, error)
    // worldLinks: closure because SetWorldNeighbors runs AFTER construction.
    worldLinks func() *worldNeighbors
    // Sub-resource surfaces, held as consumer-side interfaces so the read
    // dispatcher cannot reach the write nucleus's collaborators.
    write       entityWriteRoutes   // 7 methods of writeHandler
    attachments attachmentRoutes
    export      entityExportRoutes
}
```

Constructor takes explicit deps and rejects nil; no `app *App` parameter.

## Do NOT

- Move `scopedSortedEntities` / `applyRelationFilters` / `matchRelationFilter`
/ `resolveScope` / `handleV1EntityPosition` into this type. Five consumers
across four would-be types; absorbing it makes gantt/export/next-action depend
on `entityAPI` — a hub-and-spoke worse than today. Stays on App.
- Keep delegation methods on App so old test call sites compile. That
defeats the count. Tests call the moved handlers directly
(`handleV1ListEntities` 41×, `handleV1GetEntity` 24×, `computeEntityETag` 11×,
`handleV1EntityRelations` 10×); rewrite them to `app.entities.X` — a sed-able
diff across ~40 test files, and the bulk of this PR.

## Guard to add

A test asserting `App` has no field of type `entityReader` / `visibleReader`
after the move (or rely on the fields being deleted — then the compiler is the
guard, and `TestWorldCapableRoutesDoNotUseUngatedReader` can be re-pointed at
`entityAPI`).

## Invariants

- One `schema()` snapshot per handler.
- ACL regression pins that must stay green: everything under
`acl*_test.go`, `TestV1Views_MentionsPopulated`, the uniform-404 tests.
- `registerAPIV1Routes` becomes a table of `a.entities.*` — which also makes
the route table readable as surface-to-owner.

Ratchet `//plimsoll:max-methods` (app.go:172). After this PR App is roughly
lifecycle/setters (15) + list hub (5) + leaf handlers; the epic decides whether
a `listPipeline` type follows.
