---
id: RR-1JY8
type: review-response
title: SearchView/EntityList/useScopeNavigation will diverge
finding: Three implementations of 'read filter state from route.query' with subtly different rules. Extract a useUrlFilterSync composable or at least pure parseFilters/serializeFilters helpers that all three consumers use. The plan only mentions adding parseFilterQueryParams to filters.ts but not the watcher/writer extraction.
severity: significant
resolution: Created useUrlFilterSync composable that owns parsing/serializing/watching. EntityList, useScopeNavigation, and SearchView all consume the same composable (or its underlying parseFilterQueryParams helper). No more divergent implementations.
status: addressed
---
