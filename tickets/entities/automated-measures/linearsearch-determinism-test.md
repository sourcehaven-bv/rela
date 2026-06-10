---
id: linearsearch-determinism-test
type: automated-measure
title: 'Test: LinearSearch returns a deterministic, sorted result set'
description: 'Regression for BUG-ULU7X6: asserts repeated identical queries return the same natural-sort-ordered IDs (50 runs), a limit returns the first-N of that order, and natural-sort places REQ-2 before REQ-10. Fails if LinearSearch.Search reverts to ranging the map without sorting or truncates mid-iteration.'
kind: test
location: internal/search/linearsearch_test.go (TestLinearSearch_DeterministicOrder, TestLinearSearch_LimitReturnsFirstN, TestLinearSearch_NoMatchesEmpty)
status: active
---
