---
id: TKT-WFB6
type: ticket
title: Lua read bindings still use context.Background() — partial cancellation
kind: enhancement
priority: medium
effort: xs
status: done
---

## Problem

Lua write bindings (`luaCreateEntity`, `luaUpdateEntity`, `luaDeleteEntity`,
`luaCreateRelation`, `luaDeleteRelation`) thread the caller's ctx via
`r.callerCtx()` so audit attribution and cancellation flow through. The read
bindings still use `context.Background()`:

- `internal/lua/runtime.go:740` — `luaGetEntity`: `r.deps.Store.GetEntity(context.Background(), id)`
- `internal/lua/runtime.go:760` — `luaListEntities`
- `internal/lua/runtime.go:827` — `luaGetRelations`
- `internal/lua/runtime.go:847` — `luaTraceFrom`
- `internal/lua/runtime.go:865` — `luaTraceTo`
- `internal/lua/runtime.go:1264, 1270` — `luaSearch` (two sites)
- `internal/lua/runtime.go:1474` — `luaFindPath`

Audit-wise this is harmless (reads aren't audited). The actual user impact:
Ctrl-C during a long-running `rela.list_entities` loop or a deep `rela.trace_to`
doesn't unwind the store call. The runtime's parent ctx already carries
cancellation; the bindings just drop it.

## Pre-existing

The reviewer flagged this during the audit-log review; it's not regressed by the
audit PR, just exposed by the contrast with the now-correct write bindings.

## Proposed change

Replace `context.Background()` with `r.callerCtx()` at each read-binding site.
The helper exists (`internal/lua/runtime.go:138`) and is the right ctx source.

## Acceptance criteria

- `grep "context.Background" internal/lua/runtime.go` returns no Lua-binding hits (only auxiliary code like timeout derivation).
- A test that constructs a Lua runtime with a canceled parent ctx and calls one of the read bindings observes the store's error path (e.g. `context.Canceled`) rather than completing the lookup.
- No regression in existing Lua tests.

## Scope notes

- Read bindings can't actually cancel mid-call in some cases (the store's iterator may not honor ctx). That's a store-layer concern; this issue is just about *passing* the right ctx down.
- No documentation update needed — the Lua binding API doesn't expose ctx behavior to scripts.

---

Source: [GitHub issue #765](https://github.com/sourcehaven/rela/issues/765)
