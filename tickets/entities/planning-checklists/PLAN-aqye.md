---
id: PLAN-aqye
status: done
title: 'Planning: Add SQL query tool to MCP server'
type: planning-checklist
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

IN SCOPE:
- Add `sql_query` tool to MCP server that executes SQL queries against the rela graph
- Return structured results (columns + rows) in JSON format
- Support all SQL features the underlying go-mysql-server engine supports

OUT OF SCOPE:
- Write operations (INSERT, UPDATE, DELETE) - this is read-only
- Custom SQL functions
- Schema modification commands

**Acceptance Criteria:**

1. MCP tool `sql_query` accepts a `query` parameter with SQL
2. Returns results with `columns` array and `rows` array of arrays
3. Supports SELECT, JOIN, WHERE, GROUP BY, ORDER BY, LIMIT
4. Returns clear error messages for invalid SQL
5. Works with all entity tables and relation tables

## Research

- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Existing Solutions:**

- `internal/sqldb/query.go:23` - `Query()` function already exists that takes a graph, metamodel, and SQL string
- `internal/sqldb/query.go:16-20` - `QueryResult` struct with `Columns []string` and `Rows [][]interface{}`
- Other MCP tools follow pattern in `internal/mcp/tools.go` - register tool definition + handler
- Example: `toolExport()` in `tools.go:242-248` and `handleExport()` in `tools_export.go`

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

1. Add `sqldb` to mcp's `mayDependOn` in `.go-arch-lint.yml`
2. Create `internal/mcp/tools_sql.go` with:
   - `toolSQLQuery()` - tool definition with `query` required string parameter
   - `handleSQLQuery()` - handler that calls `sqldb.Query()` and formats result
3. Register tool in `tools.go` under "SQL tools" section
4. Result format: `{"columns": [...], "rows": [[...], [...]], "row_count": N}`

**Alternatives Considered:**
- Expose via workspace layer first: Rejected - sqldb.Query already has clean interface, no need for indirection
- Add to existing export tool: Rejected - SQL is fundamentally different from export

**Files to modify:**
- `.go-arch-lint.yml` - add sqldb to mcp dependencies
- `internal/mcp/tools.go` - register new tool
- `internal/mcp/tools_sql.go` - new file with tool definition and handler

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

- `query` parameter: SQL string from MCP client (AI assistant)
- Validation: go-mysql-server parses and validates SQL syntax
- Invalid input: Returns SQL parse error message (safe - no internal paths exposed)

**Security-Sensitive Operations:**

- Read-only queries only - go-mysql-server memory tables don't support writes
- No file system access - queries only read in-memory graph
- No authentication bypass - MCP already runs in-process with full access

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**

1. Basic SELECT: `SELECT id, title FROM requirements` → returns columns and rows
2. JOIN query: `SELECT r.id, c.title FROM requirements r JOIN implements i ON r.id = i.to_id JOIN components c ON i.from_id = c.id`
3. WHERE filter: `SELECT * FROM tickets WHERE status = 'open'`
4. Aggregation: `SELECT COUNT(*) FROM components`
5. ORDER BY + LIMIT: `SELECT id FROM features ORDER BY title LIMIT 5`

**Edge Cases:**

- Empty result set → returns `{"columns": [...], "rows": [], "row_count": 0}`
- Query with no tables in graph → returns empty result
- Very long query string → should handle gracefully
- SQL with comments → should work

**Negative Tests:**

- Invalid SQL syntax → error message with parse failure
- Non-existent table → error message "table not found"
- Missing required `query` parameter → error message "query is required"

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

- Low risk: Simple integration using existing `sqldb.Query()` function
- Mitigation for query errors: Wrap in try-catch, return user-friendly error

Effort: **S** (small) - straightforward integration

## Design Review

- [x] ~~Run `/design-review` before starting implementation~~ (N/A: simple integration, no complex design)
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** N/A - This is a simple tool registration following existing patterns. No architectural decisions required.
