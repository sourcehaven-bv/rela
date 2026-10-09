---
id: TKT-QZTROQ
type: ticket
title: Gate MCP entity and relation counts through the read ACL
kind: enhancement
priority: medium
effort: s
tags: security
completed: "2026-10-08"
status: done
---

## Description

The remote MCP reports entity and relation counts per type in the schema
resource and tool (`internal/mcp/tools_schema.go`) and in the overview prompt
(`internal/mcp/prompts.go`). They come from `gatedGraphReader.CountEntities` /
`CountRelations`, which forward to the raw store on purpose: the comment in
`internal/appbuild/appbuild.go` calls a per-type count structural and not
secret.

A caller therefore learns how many rows exist of a type it may not read, and how
many hidden rows a readable type holds. With a planned row-level read condition
(`read: [{type, when}]`), the second becomes the number of hidden rows of a type
the caller otherwise sees.

## Change

- `visibility.ScriptReader.CountEntities` counts the rows the caller may read,
using the same pushdown as `ListEntities`: deny-all is 0, allow-all is a plain
count with the face allowlist, and a composed ACL query is counted with
`store.CountMatched`. Without pushdown it counts the gated header stream.
- `visibility.ScriptReader.CountRelations` counts edges whose both endpoints are
readable, through the strict gated relation list. Relation gating has no store
pushdown yet.
- `gatedGraphReader` forwards both counts to the gated reader. The "structural"
rationale is dropped.

## Acceptance

- For a principal, the count equals the length of the gated list, for global,
face-restricted and relation-conferred read.
- A principal without read on a type gets 0.
