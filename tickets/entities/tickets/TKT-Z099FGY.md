---
id: TKT-Z099FGY
type: ticket
title: Store-side paging for list_entities filters that do not lower
kind: enhancement
priority: low
effort: m
status: backlog
---

## Description

TKT-D9UO9Y4 gave MCP `list_entities` a gated, paged store read for no filter and
for a filter that `queryplan.LowerScope` lowers to property equalities the
caller sees on every row. Every other filter still reads the whole readable type
and filters in Go (`internal/mcp/tools_entity.go#listPage.filterInGo`, a
`request-path` entry in `internal/archguard/unpagedread_allowlist_test.go`):

- `related(...)` traversals. The data-entry list pushes these into `GraphQuery.Related` after the principal's traversal gate (TKT-XKCNCL); MCP needs the same gate, from `Deps.Related.Traversals` or a `relresolve` handle.
- `current_user` references. MCP binds no query identity for `LowerScope`.
- Ordered comparisons, `~=`, `or`, `not`, and equalities on a property hidden on some rows.

Decide which of these to push down, pin each with `storetest.AssertScaleFree`,
and remove the allowlist entry.
