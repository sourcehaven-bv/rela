---
id: RR-SU4T68
type: review-response
title: 'PR-C scope wrong: GetEntity cannot carry a world, and ~50+ raw read paths bypass the handle'
finding: |-
    VERIFIED on both halves. dn37j2-plan.md §3.2-3.3 claims '~3 seams change, ~0 handlers' plus entityReader flagged. Both parts are wrong.

    (a) THE CHEAPEST SEAM STRUCTURALLY CANNOT WORK. visibleReader.getVisible (visiblereader.go:65) calls store.GetEntity(ctx, id). store.EntityReader.GetEntity (store.go:238) takes NO query struct — `GetEntity(ctx, id string)` — so there is nowhere to put a WorldScope. EntityQuery.World and GraphQuery.World exist; GetEntity has no equivalent. Fixing it means either a new store method or routing single-entity GETs through a list query — a store.Store interface change with storetest conformance implications. THE PLAN DOES NOT MAKE THIS DECISION, and it materially resizes PR-C.

    (b) filterVisible takes already-fetched entities; 'making it world-bound' is meaningless — whoever produced the candidates already read the default face.

    (c) entityReader is ~50 call sites, not 'a decision': api_v1.go (14 sites incl. computeEntityETag at :1890 — a world-bound GET with a default-face ETag is a cross-world 304 bug), relation_read_handler.go, relation_history_handler.go, history_handler.go, history_restore.go, handlers_attachment.go, views_handler.go, export.go, export_list.go, relations_direction.go, relation_visibility.go, plus write-path read-before-write sites.

    (d) WHOLE SUBSYSTEMS reach the store raw and are absent from the plan: views.go traversal; document.go:460,563 render AND its cache key (computeDocumentHash is world-blind, so a world-bound render COLLIDES with the default render in the cache); commands.go:427,440 (no read gate at all); views_handler.go:320-347 sidebar counts; helpers.go:725,729 raw Searcher.Search feeding INTO scopedSortedEntities at api_v1.go:326; visibility/tracer.go; caldav; sync.go; MCP (23 sites); Lua ReadDeps (8 sites).

    (d)'s search row is the sharpest: worldreader.NewSurface REFUSES searcher+non-default-world (surface.go:86-95), but the live list path feeds raw searcher hits straight into a seam the plan claims to fix. §3.3 names /_search, ?q= and _position but misses helpers.go:690/725 as the actual mechanism.

    FIX: re-derive PR-C. Needs (i) an enumerated, STRUCTURALLY ENFORCED set of world-safe read paths with every other path REFUSING under a non-default world; (ii) the GetEntity question resolved as a named design decision; (iii) a guard test enumerating ungated sites so the list cannot grow silently. As specified, PR-C ships a world selector that leaks the default world through dozens of endpoints.
severity: critical
status: open
---
