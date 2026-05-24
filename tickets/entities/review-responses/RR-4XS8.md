---
id: RR-4XS8
type: review-response
title: luaSearch silently swallows context.Canceled in per-hit GetEntity (defect amplified by this PR)
finding: 'internal/lua/runtime.go:1270-1273: `e, err := r.deps.Store.GetEntity(r.callerCtx(), hit.ID); if err != nil { continue }`. Before this PR, this used context.Background() (never cancels), so the only errors were ''entity not found'' (legitimate continue). After this PR, once any store honors ctx, the same continue will silently swallow context.Canceled, returning truncated results with no signal. Same shape at L762 (luaListEntities break) and L829 (luaGetRelations break).'
severity: significant
reason: Pre-existing defect that becomes load-bearing only once a store honors ctx.Err(). Out of scope per the ticket's stated scope notes ('Read bindings can't actually cancel mid-call in some cases. That's a store-layer concern; this issue is just about passing the right ctx down.'). Filed follow-up TKT-FVQ4 covering luaSearch, luaListEntities, luaGetRelations, and luaGetEntity swallow paths with a complete fix.
status: deferred
---
