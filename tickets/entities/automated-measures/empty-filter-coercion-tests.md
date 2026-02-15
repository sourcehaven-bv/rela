---
id: empty-filter-coercion-tests
kind: test
location: internal/filter/match_test.go
status: active
title: Empty filter value coercion tests
type: automated-measure
---

Unit tests in `internal/filter/match_test.go` that verify empty filter value checks (`property=""` and `property!=""`) work correctly for all property types including integers, dates, and booleans stored as native Go types.

Test function: `TestMatchEmptyFilterValueNonStringTypes`
