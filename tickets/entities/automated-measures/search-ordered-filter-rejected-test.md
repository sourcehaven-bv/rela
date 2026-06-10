---
id: search-ordered-filter-rejected-test
type: automated-measure
title: 'Test: search ordered property filters are rejected, not lexicographic'
description: 'Regression for BUG-20IGWM: asserts search.ValidateFilters rejects FilterGt/Lt/Gte/Lte with ErrOrderedFilterUnsupported (and allows eq/ne/contains/in/exists), MatchFilters defensive-non-matches an ordered op, and the store search conformance suite surfaces ErrOrderedFilterUnsupported for every backend. Fails if ordered ops revert to lexicographic comparison.'
kind: test
location: internal/search/filter_ordered_test.go (TestValidateFilters_RejectsOrderedOps, TestValidateFilters_AllowsSupportedOps, TestMatchFilters_OrderedOpIsNonMatch) + internal/store/storetest/search.go (Search/OrderedFilterUnsupported/*)
status: active
---
