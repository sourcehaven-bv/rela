---
id: RR-OMB6ID
type: review-response
title: schema.StoreCounter hardcodes context.Background(), making ctx-resolving ACL wrappers inert — invalidates the plan's stated R2 mitigation
finding: internal/schema/store_adapter.go:15 and :20 call sc.Store.CountEntities/CountRelations with context.Background(), discarding the request ctx. The whole plan rests on internal/visibility resolving the principal from ctx per call; on any collaborator that drops the ctx the gate sees no principal and the wrapper is inert. Narrowing Deps.Store therefore does NOT make analyze_schema safe, and ValidateRelationProperties additionally requires the wide concrete store.Store so the narrowing will not even compile without refactoring internal/schema.
severity: minor
resolution: 'Scope was overstated. internal/dataentry does NOT use schema.StoreCounter - it declares its own narrow ctx-taking interfaces (analyzeReader, relationCounter at analyze.go:32-41), which is the consumer-side-interface rule working as intended. StoreCounter has exactly two callers, both local/trusted: internal/mcp/tools_analysis.go:339 and internal/cli/analyze.go:670. So the context.Background() defect is confined to analyze_schema, which reports metamodel TYPE USAGE (counts per declared type), not entity rows - and type names are config, not secret, per CLAUDE.md. No broad internal/schema refactor is on the critical path. The CI check for context.Background() on gated read paths remains a good idea and is recorded as a follow-up suggestion.'
status: addressed
---

## Finding

The plan's mitigation for its highest-rated risk (R2, silent read leak) is:
"narrow `Deps` to read interfaces so raw access is *unavailable* rather than
merely discouraged." That mitigation does not hold for two collaborators.

```go
// internal/schema/store_adapter.go:10-21
type StoreCounter struct{ Store store.Store }

func (sc *StoreCounter) CountByEntityType(entityType string) int {
    n, _ := sc.Store.CountEntities(context.Background(), ...)   // ← ctx discarded
}
func (sc *StoreCounter) CountByRelationType(relationType string) int {
    n, _ := sc.Store.CountRelations(context.Background(), ...)  // ← ctx discarded
}
```

Used by `analyze_schema` at `internal/mcp/tools_analysis.go:337`.

**Why this is structural, not cosmetic.** The entire approach rests on
`internal/visibility` resolving the principal from ctx at call time
(`adapters.go:41`). A collaborator that substitutes `context.Background()`
presents the gate with *no principal*: the gate either errors or the count is
computed unrestricted. A ctx-resolving wrapper is inert here **by construction**
— and invisibly so, since it still compiles and still "has a gate."

Additionally `schema.ValidateRelationProperties(ctx, st store.Store, meta)`
(`internal/schema/validate_properties.go:54`) takes the **concrete wide**
`store.Store`, so narrowing `mcp.Deps.Store` to a read interface will not
compile for `analyze_properties`/`analyze_schema` without refactoring
`internal/schema`.

## Impact on the plan

- Pulls an **unscoped `internal/schema` refactor onto the critical path** of the
security PR. Re-estimate before committing.
- Any resolution of RR-B7ZHYO that *gates* rather than *excludes* the analyzers
must do this refactor first.

## Resolution required

1. If the analyzers are excluded from the remote surface (RR-B7ZHYO option a),
this is deferred but must be recorded so a future "just gate them" ticket does
not assume it is cheap.
2. If gated: thread the real ctx through `TypeCounter`/`StoreCounter` and change
`ValidateRelationProperties` to accept a narrow ctx-respecting reader.
3. **Systemic:** add a CI check flagging `context.Background()` inside packages
reachable from a gated read path. This bug class defeats every ctx-based ACL
design and is invisible to review; it has already happened once here.
