---
id: RR-G78J
type: review-response
title: Clear filters → ensure URL params actually deleted, not merged
finding: FilterBar emits {} for clear. handleFilter writer must actively delete filter[*] keys from query, not merge. Easy to get wrong with {...route.query, ...newFilterParams}-style merge.
severity: minor
resolution: buildQueryWithFilters strips ALL existing filter[*] keys before adding the new ones. Empty FilterState produces a query with no filter[*] entries at all.
status: addressed
---
