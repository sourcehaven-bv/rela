---
id: RR-GV4AI2P
type: review-response
title: Callers of pinned query helpers were invisible
finding: A read inside a helper taking a query parameter (listRelationsCtx, collectRelations, tracer.edges, storedHeaders) counted once against the helper; a new whole-graph caller of the helper passed the test. storedHeaders already had one (VisibleTracer.FindOrphans).
severity: significant
resolution: 'Helpers are now detected per package to a fixed point (a function whose read''s query is an unmodified parameter) and their calls are matched like reads at the call site; the helper''s own read is charged to callers. FindOrphans now shows 2 reads; helper entries were replaced by their callers'' entries. Tests: helper charged to a whole-type caller; helper with a keyed caller; helper of a helper.'
status: addressed
---
