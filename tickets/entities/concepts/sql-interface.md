---
description: Provides a SQL interface to rela graphs using dolthub/go-mysql-server. Entity types become pluralized tables (documents, components), relation types become tables with from_id/to_id columns. Supports SELECT, JOIN, WHERE, COUNT, DESCRIBE, and SHOW TABLES.
id: sql-interface
layer: server
package: internal/sqldb
status: draft
summary: MySQL-compatible SQL interface for querying rela graphs
title: SQL Interface
type: concept
---

# SQL Interface

The SQL interface allows querying rela graphs using standard MySQL-compatible SQL syntax.

## Table Mapping

- **Entity tables**: Pluralized entity type names (e.g., `documents`, `components`, `requirements`)
- **Relation tables**: Relation type names (e.g., `implements`, `affects`, `requires`)

## Entity Table Schema

Each entity table has columns:
- `id` - Entity ID
- `title` - Entity title
- `content` - Markdown body content
- All defined properties from the metamodel

## Relation Table Schema

Each relation table has columns:
- `from_id` - Source entity ID
- `to_id` - Target entity ID
- `content` - Relation content (if any)

## Supported SQL Features

- `SELECT ... FROM ... WHERE ...`
- `JOIN` operations across entity and relation tables
- `COUNT(*)` and other aggregations
- `DESCRIBE table_name`
- `SHOW TABLES`
- `LIMIT` and `OFFSET`

## Access Modes

1. **Network server** (`rela sql --port 3307`): MySQL-compatible server for external clients
2. **In-process query** (`rela query "SELECT ..."`): Direct query execution without network
