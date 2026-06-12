---
id: RR-A3V2FK
type: review-response
title: pgstore q.Filters path untested and unreachable from production — dead complexity with real risk
finding: The native impl's filter handling (ValidateFilters, MatchFilters skip, LIMIT-omission-when-filters) had zero test coverage and zero production exercise — the only caller constructs Query without Filters. The load-bearing 'omit SQL LIMIT below Go-side filters' rule was unproven.
severity: significant
resolution: 'Conformance case FiltersThenLimit (all 4 backend combos) pins filters-before-limit semantically, and a new deterministic no-DB unit test (pgstore/visiblesearch_sql_test.go TestBuildVisibleSearchSQL_LimitPlacement) pins the SQL shape directly: LIMIT pushed down without filters, omitted when Go-side filters remain. TestBuildVisibleSearchSQL_Shape additionally pins empty-scope-no-query, wildcard-no-disjunction, and distinct CTE prefixes without needing a database.'
status: addressed
---
