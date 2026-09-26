---
id: TKT-PQMBL5
type: ticket
title: Bind stdio MCP and the tracer to the default world
kind: enhancement
priority: medium
effort: m
status: backlog
---

## Description

BUG-6XTX0G binds remote MCP entity reads and searches to the operator's
`app.default_world` through `appbuild.WorldBound`. Two read surfaces still run
in the default world only, so a faced entity with no default-face row is
invisible there:

1. **Stdio `rela mcp`.** It builds its deps from `GatedReads().Reader` but uses
the raw searcher and `svc.LuaWriteDeps()` (elevated handles for
`rela.bypass_acl`), and `internal/cli` may not import `worldreader`. It needs a
world source read from `data-entry.yaml` (no grant check under NopACL) and a
binding for the Lua write deps.
2. **Tracer start node.** `trace_from`, `trace_to` and `find_path` call
`tracer.GetEntity` on the raw store, so a faced id is "not found".

## Acceptance

- `rela mcp` list, show, search and Lua reads resolve faced entities through
`app.default_world`, matching remote MCP.
- `trace_from` on a faced id starts from its world prime.
