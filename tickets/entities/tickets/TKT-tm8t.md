---
effort: m
id: TKT-tm8t
kind: enhancement
priority: medium
status: in-progress
title: Implement SQL support with tests
type: ticket
---

# Implement SQL support with tests

Complete the SQL interface implementation with proper test coverage and documentation.

## Implementation Plan

### Phase 1: Core Implementation (Done)
- [x] Create `internal/sqldb` package
- [x] Implement `Database` type wrapping rela graph
- [x] Implement `EntityTable` for entity type tables
- [x] Implement `RelationTable` for relation tables
- [x] Add `rela sql` command for network server
- [x] Add `rela query` command for in-process queries

### Phase 2: Unit Tests
- [ ] `internal/sqldb/database_test.go`
  - [ ] `TestNewDatabase` - database creation
  - [ ] `TestGetTableNames` - returns entity and relation tables
  - [ ] `TestGetTable` - entity tables by pluralized name
  - [ ] `TestGetTable` - relation tables by name
  - [ ] `TestPluralize` - pluralization rules

- [ ] `internal/sqldb/entity_table_test.go`
  - [ ] `TestEntityTableSchema` - columns match metamodel properties
  - [ ] `TestEntityTablePartitions` - row iteration
  - [ ] `TestEntityTableFiltering` - WHERE clause support

- [ ] `internal/sqldb/relation_table_test.go`
  - [ ] `TestRelationTableSchema` - from_id, to_id, content columns
  - [ ] `TestRelationTablePartitions` - row iteration

- [ ] `internal/sqldb/query_test.go`
  - [ ] `TestQuery_Select` - basic SELECT
  - [ ] `TestQuery_Where` - WHERE filtering
  - [ ] `TestQuery_Join` - JOIN across tables
  - [ ] `TestQuery_Count` - COUNT aggregation
  - [ ] `TestQuery_Describe` - DESCRIBE table
  - [ ] `TestQuery_ShowTables` - SHOW TABLES

### Phase 3: Integration Tests
- [ ] Test with real metamodel and entities
- [ ] Test complex JOIN queries
- [ ] Test error handling for invalid SQL

### Phase 4: Network Server Tests
- [ ] Server startup/shutdown
- [ ] MySQL client connection test
- [ ] Query execution over wire protocol

### Phase 5: Documentation
- [ ] Update CLAUDE.md with SQL commands
- [ ] Add examples to CLI help text

## Test Data Setup

Create test fixtures with:
- Multiple entity types (at least 3)
- Multiple relation types (at least 2)
- Various property types (string, enum, integer, boolean)
- Enough entities for meaningful JOIN tests

## Acceptance Criteria

1. All unit tests pass
2. Coverage meets baseline requirements
3. `rela query` works for all documented SQL features
4. `rela sql` server accepts MySQL client connections
5. DESCRIBE and SHOW TABLES return correct schema info
