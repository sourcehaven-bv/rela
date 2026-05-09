---
id: RR-QR0X7
type: review-response
title: Nil deps cause panic instead of useful error
finding: luaMdEntityRefs accesses r.deps.Meta and r.deps.Store unconditionally. If the runtime was built with empty ReadDeps (as the markdown test helper does), invoking entity_refs panics with nil-pointer deref. Should produce a Lua error or document the requirement.
severity: significant
resolution: luaMdEntityRefs now nil-checks r.deps.Meta and r.deps.Store at entry, raising a Lua error with the canonical prefix. Added test TestMdEntityRefs_NilDeps verifying the error is raised on a runtime constructed via newMdTestRuntime (which uses empty ReadDeps).
status: addressed
---

# Finding

`luaMdEntityRefs` accesses `r.deps.Meta.EntityTypes()` (line 1668),
`r.deps.Store.ListEntities` (line 1674), and `r.deps.Meta.GetEntityDef` in
`parseEntityRefsOpts`. None are nil-checked.

The markdown test helper `newMdTestRuntime` constructs a runtime with empty
`ReadDeps{}`. Invoking `entity_refs` on that runtime panics.

# Resolution

Guard explicitly:

```go
if r.deps.Meta == nil || r.deps.Store == nil {
    ls.RaiseError("rela.md.entity_refs: requires a runtime with metamodel and store")
    return 0
}
```

Add a test that calls `entity_refs` on a runtime constructed without deps —
expect a Lua error, not a panic.
