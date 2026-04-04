---
description: Provides SQL interface to rela graphs via dolthub/go-mysql-server. Supports network server mode (rela sql) and in-process queries (rela query).
id: FEAT-1hca
priority: medium
status: in-progress
summary: Query rela graphs using MySQL-compatible SQL syntax
title: SQL Support
type: feature
---

# SQL Support

Query rela graphs using familiar SQL syntax instead of the graph-specific API.

## Capabilities

### Table Mapping
- Entity types become tables with pluralized names (e.g., `documents`, `requirements`)
- Relation types become tables with `from_id`, `to_id`, `content` columns

### Supported SQL
- `SELECT ... FROM ... WHERE ...` with all standard operators
- `JOIN` across entity and relation tables
- `COUNT(*)` and aggregations
- `DESCRIBE table_name` - show table schema
- `SHOW TABLES` - list available tables

### Access Modes

1. **Network Server** (`rela sql --port 3307`)
   - MySQL-compatible wire protocol
   - Connect with any MySQL client (mysql, mysqlsh, DBeaver, etc.)
   - Useful for exploration, dashboards, external tools

2. **In-Process Query** (`rela query "SELECT ..."`)
   - Direct query without network overhead
   - Supports `--format json` for scripting
   - Useful for CLI automation, scripts

## Implementation

Uses [dolthub/go-mysql-server](https://github.com/dolthub/go-mysql-server) which provides:
- Full MySQL parser and query planner
- In-memory table implementation
- MySQL wire protocol server

## Example Queries

```sql
-- List all documents
SELECT id, title FROM documents

-- Functions implementing requirements
SELECT f.id, f.title, r.id as req_id, r.title as req_title
FROM functions f
JOIN implements i ON f.id = i.from_id
JOIN requirements r ON i.to_id = r.id

-- Count entities by type
SELECT COUNT(*) FROM components WHERE status = 'active'

-- Describe a table
DESCRIBE requirements
```

## Testing Requirements

1. **Unit Tests**
   - Table schema generation from metamodel
   - Row iteration for entities and relations
   - Property type mapping to SQL types

2. **Integration Tests**
   - Basic queries: SELECT, WHERE, LIMIT
   - JOIN queries across entity/relation tables
   - Aggregations: COUNT, GROUP BY
   - Schema introspection: DESCRIBE, SHOW TABLES

3. **Network Server Tests**
   - Server startup and shutdown
   - MySQL client connectivity
   - Concurrent connections

## Current Status

- [x] POC implementation complete
- [x] Entity and relation tables working
- [x] Network server mode (`rela sql`)
- [x] In-process query mode (`rela query`)
- [ ] Unit test coverage
- [ ] Integration test coverage
- [ ] Documentation
