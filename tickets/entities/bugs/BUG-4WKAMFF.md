---
id: BUG-4WKAMFF
type: bug
title: Search shows a property filter error as an empty result
description: A filter error in the data-entry search is read as no match so the user sees an empty list instead of the error.
priority: medium
status: backlog
---

## Description

`queryService.matchesPropertyFilters` (internal/dataentry/queryservice.go)
returns `err == nil && matched`, so any `filter.MatchAll` error becomes "no
match". A search such as `prop:niveau=typo` (unknown enum value) or
`prop:niveau<kritiek` (unsupported operator) returns an empty list with no
message. The search also ignores `sq.ParseErrors`.

This is how BUG-8SROSON (regex on enum) stayed hidden: the user saw an empty
result, not an error.

## Fix

Surface the first filter error, as the v1 list filter does
(`TestV1FilterUnknownOperator`): a 400 with the message, or a warning next to
the result. Decide which for query cards, where one bad card must not break a
dashboard.
