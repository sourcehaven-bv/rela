---
effort: s
id: TKT-elbr
kind: enhancement
priority: medium
status: review
title: Add SQL query tool to MCP server
type: ticket
---

Expose a `sql_query` tool in the MCP server that allows AI assistants to execute SQL queries against the rela graph. This complements existing MCP tools by enabling complex JOINs, aggregations, and flexible filtering using standard SQL syntax.

## Background

The `internal/sqldb` package already provides SQL query functionality via:
- `sqldb.Query()` for in-process queries
- Entity tables with pluralized names (requirements, functions, etc.)
- Relation tables (implements, belongs-to, etc.)

## Scope

- Add `sql_query` tool to MCP server
- Return results in structured format (columns + rows)
- Support all SQL features the underlying engine supports
