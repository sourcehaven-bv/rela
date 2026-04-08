---
id: RR-R68T
type: review-response
title: EntityList duplicates filter serialization logic
finding: 'EntityList.vue''s queryParams builder hand-rolls the bracket-format key construction for user filters, duplicating logic that already lives in buildQueryWithFilters. If the wire format ever changes, both call sites must be updated in lockstep. Fix: extract a shared helper toFilterApiParams(state: FilterState): Record<string, string | string[]> and use it from EntityList AND useScopeNavigation (which has the same duplication).'
severity: minor
resolution: filters.ts adds filterStateToApiParams(state) helper that serializes a FilterState into flat bracket-format API params. EntityList.vue queryParams builder and useScopeNavigation.ts loadScopeNav both now route through this helper, so the wire format has a single source of truth. 5 new unit tests cover the happy path, default/explicit '=' omission, non-default operators, and empty-value skipping.
status: addressed
---
