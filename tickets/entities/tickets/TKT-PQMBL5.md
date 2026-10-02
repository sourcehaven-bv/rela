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
2. **Tracer.** `trace_from`, `trace_to` and `find_path` pass the existence
check through the world but then traverse bare-id edges in the default world.
3. **Validator and analyze tools.** `analyze_validations` reads the default
world (the validator is built over the unbound reader), while
`analyze_properties`/`analyze_unique` now see world primes only and
`analyze_cardinality` uses AllStates. Decide which set each tool checks.
4. **Lua `readFace`.** It sends `IDs` + `FaceIn` without `AllStates`, so the
world is stamped on it and a face outside the chain is not found.
5. **ACL pushdown.** For a typed query, `visibility.listPushdown` drops
`q.IDs` and `q.AllStates` and replaces `q.FaceIn`; `BoundReader` works around it
with type-less queries and an id check.

## Acceptance

- `rela mcp` list, show, search and Lua reads resolve faced entities through
`app.default_world`, matching remote MCP.
- `trace_from` on a faced id starts from its world prime.
- The analyze tools check a documented, consistent entity set.
- A typed `ListEntities` through the gated reader honours `IDs`, `AllStates`
and a caller's `FaceIn`.
