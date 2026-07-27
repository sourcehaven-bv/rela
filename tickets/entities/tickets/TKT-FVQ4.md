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

## CONFIRMED EMPIRICALLY (2026-07-27)

Reproduced against current `develop`, so this is a defect **today**, not one
latent until a store honors `ctx.Err()`. A `VisibleReader` that yields 3
entities then `context.Canceled` (exactly the shape
`visibility.ScriptReader.ListEntities` produces — it faithfully propagates via
`yield(nil, err); return`) gives:

```text
script error: <nil>
script saw:   "entities=3 relations=0"
```

Two distinct failures, the second worse than the ticket originally described:

- `list_entities` — 3 of N entities, **no error**. The script sees a plausible
short list and cannot tell it from the real answer.
- `get_relations` — **0 relations, no error**, which is indistinguishable from
"no such edges". A script asking "does anything depend on this?" gets a
confident *no* from a query that actually failed. That is the shape that turns
into a wrong write.

The line numbers in the section above have drifted; the live sites are
`runtime.go:888-893` (`luaListEntities`), `:957-963` (`luaGetRelations`), and
`:1416-1426` (`luaSearch`).

## Related: the elevated bindings already do this correctly

TKT-ACSBSA's `admin.list_entities` / `admin.get_relations` raise on iterator
error rather than breaking. The gated bindings are now the inconsistent ones —
the same asymmetry TKT-9FKX8X just closed for filter typing, in the identical
loop. Whoever picks this up should fix both surfaces together.

## Why it was OK before

Until TKT-WFB6 these bindings called `context.Background()`, which never
cancels, so `continue`/`break` only handled real (non-cancellation) errors.
TKT-WFB6 threaded the parent ctx through; the swallow paths are now load-
bearing for cancellation reporting.

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

Open question worth deciding when picked up: whether a NON-cancellation store
error should also raise. The demo above shows `get_relations` returning a
confident empty list on a generic failure too, so restricting the fix to
cancellation would leave the more dangerous half of the defect in place.

## Acceptance criteria

- Tests that simulate a canceled ctx (using a fake store/searcher that
returns `context.Canceled` from its iterator/method) assert that the binding
raises a Lua error rather than returning a truncated result.
- "Entity not found" errors still produce skip-and-continue behavior.

## Scope notes

- Discovered during code review of TKT-WFB6 (see RR-4XS8).
- Independent of whether/when fsstore honors ctx; the swallow path is a
defect today regardless.
- Overlaps TKT-YWDGZD (unbounded `list_entities`), which rewrites these same
two loops. Doing them together avoids touching the loop twice.
