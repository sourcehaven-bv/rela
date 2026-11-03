---
id: TKT-MCPCNV
type: ticket
title: MCP convert_entity and list_conversions tools
kind: enhancement
priority: medium
effort: s
started: "2026-10-24"
completed: "2026-10-24"
status: done
---

## Description

MCP clients could not run the schema-declared conversions the web app offers.
Add `list_conversions` and `convert_entity`. Both go through
`entitymanager.Converter`, so the `convert:` grant, read checks and field gate
are the ones `POST /api/v1/_conversions/` applies. Dropping relations needs
`confirm: true`, as `run_action` does for a confirmed action.

## Acceptance Criteria

- [x] `list_conversions` lists the offers on an entity with the ACL verdict.
- [x] `convert_entity` reports required fields, face choices and dropped relations, and supports `dry_run`.
- [x] Dropping relations requires `confirm: true`.
- [x] A caller without `convert:` is refused; a hidden entity reads as not found.
- [x] Tests in `internal/mcp/tools_conversions_test.go`.
