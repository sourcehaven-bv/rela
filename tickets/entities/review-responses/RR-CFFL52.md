---
id: RR-CFFL52
type: review-response
title: MCP resources surface (rela://entity, rela://relation) is a second ungated read path, and leaks entity type
finding: MCP resources (rela://entity/{type}/{id}, rela://relation/{from}/{type}/{to}) read raw via deps.Store at resources.go:89 and :124, bypassing the tool-level read gating entirely. resources.go:94 additionally discloses an entity's real type on mismatch, creating an existence-and-type oracle.
severity: critical
status: open
---

## Finding

Design review of the remote-MCP plan (TKT-UIR41P / PLAN-O8KMBQ) found that the
plan's read-gating story covered only the *tool* handlers. The MCP **resources**
surface is a fully parallel read path to the same data:

- `internal/mcp/resources.go:89` — `handleReadEntity` for
`rela://entity/{type}/{id}` calls `s.deps.Store.GetEntity(ctx, id)` raw.
- `internal/mcp/resources.go:124` — `handleReadRelation` for
`rela://relation/{from}/{type}/{to}` calls `st.GetRelation(...)` raw.

Gating the tools alone would leave remote callers able to read any entity or
relation through the resource URI instead.

Additionally, `resources.go:94` returns:

```go
return nil, fmt.Errorf("entity %s is type %s, not %s", id, e.Type, entityType)
```

This discloses an entity's **real type** to a caller who may not be permitted to
read it at all — an existence-and-type oracle. `docs/acl-security.md` and the
row-level rule in CLAUDE.md require that a hidden entity be indistinguishable
from a nonexistent one.

## Resolution required

1. Route resource handlers through the same injected visibility-wrapped reader
as the tools.
2. Collapse the type-mismatch branch into the same indistinguishable not-found
response used for a denied/absent entity.
3. Extend AC #3 to cover the resource surface, not just tools.
