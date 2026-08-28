---
id: RR-596TYU
type: review-response
title: Sync relation fetch must fail closed on unresolvable source (bare visibleRelationMeta fails OPEN on nil source)
finding: 'The planned relation redaction reuses visibleRelationMeta(ctx, from, relType, meta). Verified in code: visibleRelationMeta returns meta UNCHANGED when from is nil (affordances.go:960), i.e. it fails OPEN. The sync relation fetch resolves the source via a raw GetEntity(from) that can miss (source deleted between manifest-read and fetch, or cascade lag) — exactly the case permitsSyncReadRelation already tolerates by leaving fromType empty. If the row gate permits and the source is then nil, bare visibleRelationMeta returns hidden meta in the clear. Concrete leak: relation assigned TKT-1 -> alice with hidden meta ''rate''; TKT-1 deleted between feed and fetch; source no longer loads; naive call returns ''rate'' unredacted. This is the same fail-open the web incoming path added visibleRelationMetaIncoming to prevent (fail-closed, drops the whole meta map when the peer is gone).'
severity: significant
resolution: 'Fixed in the affordanceSyncRedactor.visibleRelationMeta adapter (sync_handler.go): it resolves the edge source via getEntity and returns EMPTY meta when the source is not live, mirroring serveRelationHistoryVersion. Bare affordanceService.visibleRelationMeta (which fails open on nil source) is never called directly. Pinned by TestSync_GetRelation_SourceGone_FailsClosed.'
status: addressed
---

## Finding (design-review C2)

The sync relation fetch must NOT call bare `visibleRelationMeta` on a
possibly-nil source. It must mirror the fail-**closed** behavior of
`visibleRelationMetaIncoming` (affordances.go:993-1008) / the relation-history
handler (`relation_history_handler.go:~301-304`): resolve the source, redact
only when `src, live := getEntity(from)` succeeds, and emit **empty meta** when
the source is not live — never raw meta.

Reference implementation already exists: the relation-history handler does
exactly this standalone `getEntity(snap.From)` + `visibleRelationMeta` +
empty-on- dead-source pattern (from the TKT-B1F5Q1 live-world work). Copy its
structure.

Also reconcile the row-gate/redaction source agreement:
`permitsSyncReadRelation` gates on `from` with `fromType=""` on a missing
source; make sure the empty-`from`-type gate path and the nil-source redaction
path agree (gate says yes ⇒ redaction must fail closed, not fall through to raw
meta).

## Recommended resolution

Add a small fail-closed wrapper for the sync single-relation fetch modeled on
the relation-history handler: source-live ⇒ per-field redact; source-gone ⇒
empty meta (serve the edge with no meta rather than raw meta). Pin with a test:
`TestSync_GetRelation_SourceGone_FailsClosed`.
