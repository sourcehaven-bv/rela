---
id: RR-FTJUUE
type: review-response
title: trace_from/trace_to/find_path do a raw GetEntity pre-flight probe, so a gated tracer still leaks hidden-vs-absent
finding: 'tools_trace.go:42 (and the from/to pair around :72-78) call s.deps.Store.GetEntity directly before delegating to the tracer, returning ''entity not found'' only on a store miss. Swapping Deps.Tracer for visibility.VisibleTracer does not help: the raw probe already distinguished a hidden entity from an absent one, defeating AC #3''s indistinguishability requirement.'
severity: critical
status: open
---

## Finding

`internal/mcp/tools_trace.go:42`:

```go
if _, getErr := s.deps.Store.GetEntity(ctx, id); getErr != nil {
    return mcp.NewToolResultError("entity not found: " + id), nil
}
result := traceFn(s.deps.Tracer, id, maxDepth)
```

and the same pattern for both endpoints of `find_path` (`:72-78`).

The plan assumed `trace_*`/`find_path` were covered by swapping `Deps.Tracer`
for `visibility.VisibleTracer` (which does implement `TraceFrom`/`TraceTo`/
`FindPath`). But the **raw pre-flight probe runs first**, and it answers a
different question: it distinguishes "the store has no such entity" from "the
store has it." For a hidden entity the probe *succeeds*, so the caller gets a
trace result (empty/pruned) rather than the same "not found" an absent entity
produces — a clean existence oracle.

This directly contradicts an edge case the plan asserts:

> "Entity hidden *between* the gate probe and the read → still no leak, since
> the gate runs first."

Here the gate does **not** run first; a raw store probe does. The `getVisible`
gate-first ordering (`internal/dataentry/visiblereader.go:35`), which the plan
cites as the pattern to copy, exists precisely to prevent this.

## Resolution required

Replace the pre-flight probe with a gate-first lookup so hidden and absent
produce byte-identical responses. Add this case explicitly to the AC #3 test
matrix (trace and find_path on a hidden id vs. a nonexistent id).
