---
id: RR-2RK4
type: review-response
title: callerCtx() called multiple times per operation — should capture once
finding: CLAUDE.md says 'capture state once per operation.' luaSearch calls r.callerCtx() twice (L1264, L1270). Cheap nil-check so no correctness issue, but the spirit of the rule favors `ctx := r.callerCtx()` at the top. Also makes the C1 fix simpler — the test can assert both call sites used the *same* ctx.
severity: minor
resolution: luaSearch (runtime.go:1262-1278) now captures `ctx := r.callerCtx()` once before the iterator loop and reuses it for both the Search call and the per-hit GetEntity. Matches the 'capture state once per operation' CLAUDE.md rule. Other bindings still call r.callerCtx() once at the call site — fine, since they each invoke a single collaborator method.
status: addressed
---
