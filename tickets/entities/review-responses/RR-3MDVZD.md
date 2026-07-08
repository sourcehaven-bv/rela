---
id: RR-3MDVZD
type: review-response
title: Wire format is filter[<rel>], not filter_<rel>
finding: The plan stated the query-param key stays `filter_<relation>=<title>`. That is wrong for the SPA/v1 path. Backend `parseRelationFilterKey` (internal/dataentry/api_v1.go:418) only recognizes the bracket form `filter[<rel>]`; `filter_<rel>` is never parsed as a relation filter and silently matches nothing. The SPA emits bracket form everywhere (filters.ts filterStateToApiParams:188 / buildQueryWithFilters:161), and relation_filter_test.go:139 proves `filter[verantwoordelijk_voor]={title}` is the working format. The plan's error came from citing `FilterControl.QueryParamKey()` (config.go:257, returns `filter_`+key), which is DEAD LEGACY for the old server-side template path, not the v1 API.
severity: critical
resolution: 'Plan corrected: wire format is `filter[<relation>]=<title>`. Mechanism is already right — FilterBar keys localFilters/buildState by the bare relation name (control.property || control.relation), so a relation filter already emits the correct bracket form today via free text. The selector just sets localFilters[relation]=<title> and calls handleFilterChange(); nothing about the wire format changes.'
status: addressed
---
