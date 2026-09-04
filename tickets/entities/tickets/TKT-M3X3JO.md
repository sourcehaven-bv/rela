---
id: TKT-M3X3JO
type: ticket
title: 'Extract graphReads off lua.Runtime: the read-out bindings holding only the ACL-gated reader (46 → 36)'
kind: refactor
priority: medium
effort: s
tags:
    - tech-debt
    - security
status: ready
---

Sub-ticket of [[TKT-N0IKN9]], continuing the `lua.Runtime` arc after
[[TKT-4WBLG6]] and [[TKT-DOPCTI]]. Clears the 40 line by itself.

## The established pattern (follow it exactly)

Four prior extractions are recorded in the `Runtime` struct doc at
`internal/lua/runtime.go:64-101` — append the new entry there. Invariant shape
(`ai.go:49`, `http.go:113`, `cache.go:260`, `urls.go:25`, `markdown.go:77-120`):

1. Small unexported struct in the cluster's own file holding **only the deps
its bindings touch — never `*Runtime`**.
2. The `registerXBindings` seam STAYS on Runtime, constructs the struct from
`r`'s fields, and does the `SetField` wiring.
3. Mutable runtime state is passed as **closures** (`cacheBindings.scriptPath
func() string`, `mdEntityRefs.ctx func() context.Context`), not captured.
4. Bindings register as **method values**, never inline closures —
`contextcheck` otherwise demands a ctx at every `NewReader`/`NewWriter`
(rationale at mail.go:87-96).
5. A held `*lua.LState` may be used only for `NewTable`; `Push`/`RaiseError`/
`CheckX` use the `ls` parameter, because coroutines invoke bindings on a thread
LState (markdown.go:79-86).

## What moves (10 methods → `internal/lua/graphreads.go`)

`reader` (runtime.go:1123 — the TKT-ZF2DTV choke point), `luaGetEntity` (:1132),
`luaListEntities` (:1154), `luaGetRelations` (:1234), `luaTraceFrom` (:1268),
`luaTraceTo` (:1286), `luaSearch` (:1731), `luaFindPath` (:2361 — currently
misfiled among the write bindings), `luaGetEntityTypes` (:2409),
`luaGetRelationTypes` (:2448). `luaSortEntities` (:2487) is pure LState — the
same category as `urlHelpers`; move it too or make it a free function.

## Shape

```go
// graphReads implements the read-OUT rela.* bindings. reader is the SOLE
// read handle: there is no store field beside it and no *Runtime
// back-pointer, so readerOrRaise is a choke point that cannot be reached
// around (TKT-ZF2DTV).
type graphReads struct {
    reader   EntityReader   // Nil: accepted at construction, DENIED at call — raises, never falls back (RR-X9NVHI)
    tracer   tracer.Tracer
    searcher search.Searcher // Nil: accepted — rela.search raises "not available"
    meta     *metamodel.Metamodel
    ctx      func() context.Context // Runtime.callerCtx, read at CALL time
}
```

`registerReadBindings` (runtime.go:842) becomes construction +
`r.L.SetField(rela, "get_entity", r.L.NewFunction(g.luaGetEntity))` etc.

## Risks (MEDIUM — this is the ACL surface)

- Renaming/moving `reader()` must not turn "nil ⇒ raise" into "nil ⇒ fall
through". Pinned by `aclreads_test.go`, `acl_bypass_reads_test.go`,
`list_entities_bound_test.go`.
- `luaSearch`'s hydration-is-the-gate behaviour (:1751-1770) incl. the
`errors.Is(err, store.ErrNotFound)` silent-skip vs `slog.Warn` split (RR-QSP6X2)
moves verbatim.
- `relationQuery` is shared with the elevated `admin.get_relations`
(RR-D7KXKV) — leave that free function where it is.
- `mdEntityRefs` (markdown.go:104) already holds `deps.VisibleReader`
directly, bypassing `reader()`. Pre-existing asymmetry: note, don't fix here.
- `capabilityguard_test.go` greps package source for the AI/HTTP/write_file
gates; this PR touches none of its needles.

## Done when

Receiver + field renames only, no logic edits. Struct doc entry `46 → 36
(TKT-…)` with the reason the cluster was separable. Ratchet
`//plimsoll:max-methods` at runtime.go:102 to 36. `go test ./internal/lua/...`,
`just plimsoll`, `just comment-lint`, `just coverage-check`.
