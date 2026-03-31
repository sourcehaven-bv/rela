---
effort: m
id: TKT-7g7x
kind: refactor
priority: low
status: ready
title: Replace hardcoded ID comparisons in test assertions
type: ticket
---

## Problem

Many test files compare against hardcoded ID strings and property values in assertions, which couples the test to specific values rather than referencing the entity being tested. This also prevents the use of randomized test data.

Example of problematic patterns:
```go
entity := testutil.Entity("ticket").ID("T-001").Build()
// ... later ...
if rel.From != "T-001" {  // hardcoded - should use entity.ID

// Also:
if toCreate.Properties["title"] != "Checklist for T-001" {  // should use entity.ID
```

## Goals

1. **Enable random test data**: By removing hardcoded value assertions, tests can use `EntityFor()` with auto-generated random IDs and properties
2. **Clarify test intent**: Only specify values that are actually being tested
3. **Reduce boilerplate**: Let the test fixtures handle default values

## Scope

~130+ instances across the codebase fall into these categories:

### 1. Can use entity reference (IN SCOPE)
Where entity is in scope, use `entity.ID` instead of hardcoded string:
```go
// Before
if rel.From != "T-001" { ... }
// After  
if rel.From != entity.ID { ... }
```

### 2. Needs local constants (IN SCOPE)
Where IDs are passed to helper functions, extract to local variables:
```go
// Before
mustCreate(t, ws, "requirement", CreateOptions{ID: "REQ-001", ...})
if rel.From != "REQ-001" { ... }
// After
reqID := "REQ-001"
mustCreate(t, ws, "requirement", CreateOptions{ID: reqID, ...})
if rel.From != reqID { ... }
```

### 3. Interpolated property values (IN SCOPE)
Where assertions check interpolated values, construct expected value from entity:
```go
// Before
if toCreate.Properties["title"] != "Checklist for T-001" { ... }
// After
if toCreate.Properties["title"] != "Checklist for "+entity.ID { ... }
```

### 4. Preserved property assertions (IN SCOPE)
Where tests verify a property wasn't changed, compare against original entity:
```go
// Before
if updated.Properties["title"] != "Original Title" { ... }
// After
if updated.Properties["title"] != original.Properties["title"] { ... }
```

### 5. Ordering tests (OUT OF SCOPE)
Where specific IDs are used to verify sort order, hardcoded strings are appropriate since the test is verifying deterministic ordering.

### 6. Parse/read tests (OUT OF SCOPE)
Where the ID comes from parsed content (files, fixtures), hardcoded strings are appropriate since the test is verifying the parser reads specific values.

### 7. Automation trigger values (OUT OF SCOPE)
Where specific values trigger automation rules (e.g., status="in-progress"), hardcoded values are appropriate since that's what's being tested.

## Files with most instances

From grep analysis:
- `internal/mcp/convert_test.go` (~25 instances)
- `internal/dataentry/helpers_test.go` (~15 instances)
- `internal/dataentry/handlers_test.go` (~10 instances)
- `internal/workspace/workspace_test.go` (~5 instances)
- `internal/rename/rename_test.go` (~10 instances)
- `internal/automation/engine_test.go` (~10 instances)
- `internal/lua/runtime_test.go` (~15 instances)

## Acceptance Criteria

- [ ] Category 1-4 instances use entity/property references where applicable
- [ ] Tests that don't need specific IDs can use `EntityFor()` with random IDs
- [ ] All tests pass after refactoring
- [ ] No new test failures introduced
- [ ] Test intent is clearer (only explicitly set values that matter for the test)
