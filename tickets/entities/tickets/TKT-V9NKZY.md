---
id: TKT-V9NKZY
type: ticket
title: 'Extract graphWrites off lua.Runtime: mutation bindings + bypass_acl elevation as a writer-only type (36 → 29)'
kind: refactor
priority: medium
effort: m
tags:
    - tech-debt
    - security
status: ready
---

Sub-ticket of [[TKT-N0IKN9]], `lua.Runtime` arc. Follows the graphReads
extraction; textually independent of it (touches `registerWriteBindings`, not
`registerReadBindings`), so it can be developed in parallel — only the struct
doc comment at runtime.go:64-101 will conflict.

## Why a separate type from graphReads

The read/write split is the same one `ReadDeps`/`WriteDeps` draws (CLAUDE.md
capability bundles): a reader runtime never constructs this, so read-only code
cannot reach a mutator by holding the wrong struct. Follow the established
pattern listed in the graphReads ticket (deps-only struct, seam stays on
Runtime, closures for mutable state, method values, `ls` parameter for
Push/Raise).

## What moves (7 methods → `internal/lua/graphwrites.go`)

Writes: `luaCreateEntity` (runtime.go:1791), `luaUpdateEntity` (:1833 —
PatchEntity), `luaDeleteEntity` (:2300), `luaCreateRelation` (:2319),
`luaDeleteRelation` (:2341). Elevation: `luaBypassACL` (:1918),
`newElevatedHandle` (:2023). `readUsage` (:2001) and `recordElevatedReads`
(:2016) are already Runtime-free and move as-is.

`luaWriteFile` stays behind under its `r.caps.WriteFile` gate — see the
`capabilityguard_test.go` constraint below.

## Shape

```go
type graphWrites struct {
    manager Mutator // never nil on a writer runtime — NewWriter panics without it (runtime.go:400)
    // bypass_acl. Both nil ⇒ binding not registered. Nil elevatedReader with
    // non-nil elevatedManager leaves the admin read methods PRESENT and
    // RAISING (TKT-Y3JVFK) — move that comment with newElevatedHandle.
    elevatedManager   Mutator
    elevatedReader    EntityReader
    elevationRecorder ElevationRecorder
    ctx func() context.Context
}
```

`registerWriteBindings` (runtime.go:889) constructs it. Keep the
`r.deps.ElevatedManager != nil || r.deps.ElevatedReader != nil` condition
(runtime.go:803) textually where it is to avoid re-reviewing it.

## Risks (MEDIUM-HIGH — the elevation boundary)

- The `live` invalidation defer (runtime.go:1941-1949) and audit-rides-the-
same-defer property move intact. Pinned by `acl_bypass_test.go`,
`audit_spoofing_test.go`.
- `isEntityNotFound` structural check in `luaUpdateEntity` (:1863) — do not
let the move turn it back into a string match.
- `capabilityguard_test.go:28-46` greps all package source: exactly one
`SetField(rela, "write_file"` within 200 bytes of `if r.caps.WriteFile {`. Leave
that block untouched.
- `internal/dataentry/elevated_document_test.go:315,327` constructs
`lua.NewWriter` directly and relies on the panic-without-EntityManager.
- If the diff feels large, split elevation into its own `elevatedBindings`
PR — but only then; two types where one suffices is its own cost.

## Explicitly NOT in scope

- Extracting a `runner`/`executor` for `RunFile`/`RunString`/`applyTimeout`/
`pcallWithCapture`: that is the VM lifecycle and Runtime's identity; a facade
delegating method-for-method is not a decomposition.
- Extracting the `registerX` seams. All prior extractions kept them.
- An `outputBindings` type (`luaPrint`/`luaOutput`/`luaWriteFile`, −3): marginal
once the two above land; the `print` override at runtime.go:438 runs before
`registerBindings`, which makes it awkward. Skip unless headroom is wanted.

Ratchet `//plimsoll:max-methods` (runtime.go:102) to 29; append the struct doc
entry.
