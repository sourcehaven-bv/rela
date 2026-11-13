---
id: TKT-D9UO9Y4
type: ticket
title: Gated paged reads for MCP list_entities and search with no text
kind: enhancement
priority: medium
effort: m
status: backlog
---

## Description

TKT-FED6FMM left these `request-path` entries in
`internal/archguard/unpagedread_allowlist_test.go`:

- `internal/mcp/tools_entity.go#Server.handleListEntities`: reads the whole type (or every type), then filters, sorts and applies offset and limit in Go.
- `internal/search/index.go#Service.listAll` and `#Service.scanFaces`: a search with no text reads every entity in the world, then filters by type and properties.

Both need a gated, paged read with a gated total. `mcp.GraphReader` has no paged
list, and its `CountEntities` is a structural counter that may be raw, so it
cannot give the total. Add a gated `ListEntitiesPage`-style read (or route
through `visibility.ScriptReader` pushdown with `Limit`/`Offset` and
`CountMatched`), push lowerable filters into `GraphQuery.Props`, and pin each
with `storetest.AssertScaleFree`.
