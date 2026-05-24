---
id: TKT-FVQ4
type: ticket
title: Lua iterator bindings silently swallow context.Canceled in error paths
kind: enhancement
priority: medium
effort: s
status: backlog
---

## Problem

Three Lua read bindings catch errors from the underlying iterator / per-item
fetch and either `continue` past them or `break` out, returning whatever partial
result has accumulated so far. This is fine when the only possible error is
"entity not found" (concurrent deletion racing with a search hit) — that's a
legitimate skip. But it becomes silent data corruption once any store honors
`ctx.Err()`:

- `internal/lua/runtime.go:1270-1273` — `luaSearch` per-hit `GetEntity`:
`if err != nil { continue }`. Canceled mid-search → truncated result list, zero
signal to the script.
- `internal/lua/runtime.go:760-764` — `luaListEntities` iterator: `break`
on any error. Canceled mid-iteration → truncated result.
- `internal/lua/runtime.go:826-832` — `luaGetRelations` iterator: same
`break`-on-error pattern. Same risk.

`luaGetEntity` at L740-744 returns Lua `nil` on any error, making a canceled ctx
indistinguishable from "entity not found" — also a silent data-shape change.

## Why it was OK before

Until TKT-WFB6 these bindings called `context.Background()`, which never
cancels, so `continue`/`break` only handled real (non-cancellation) errors.
TKT-WFB6 threaded the parent ctx through; the swallow paths are now load-
bearing for cancellation reporting.

## Why it isn't OK now

Once fsstore (or any production store) honors `ctx.Err()` — likely follow- up
work to TKT-WFB6 — a `Ctrl-C` mid-search will surface as "search returned 4 hits
but the script expected 50" with no observable error. Audit logs and 5-whys
analyses will spend weeks tracking it down before the cause is found.

## Proposed change

At each swallow site, check for `context.Canceled` / `context.DeadlineExceeded`
specifically and propagate, distinct from the other errors:

```go
e, err := r.deps.Store.GetEntity(ctx, hit.ID)
if err != nil {
    if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
        ls.RaiseError("search canceled: %s", err.Error())
        return 0
    }
    continue // entity removed between index and read
}
```

Apply analogous treatment to `luaListEntities` and `luaGetRelations` iterator
break sites, and to `luaGetEntity` (Lua `nil` for "not found" is fine but
cancellation should `ls.RaiseError`).

## Acceptance criteria

- Tests that simulate a canceled ctx (using a fake store/searcher that
returns `context.Canceled` from its iterator/method) assert that the binding
raises a Lua error rather than returning a truncated result.
- "Entity not found" errors still produce skip-and-continue behavior.

## Scope notes

- Discovered during code review of TKT-WFB6 (see RR-4XS8).
- Independent of whether/when fsstore honors ctx; the swallow path is a
defect today regardless.
