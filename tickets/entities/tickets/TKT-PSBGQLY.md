---
id: TKT-PSBGQLY
type: ticket
title: Push the remaining structured search shapes into the store
kind: enhancement
priority: low
effort: m
status: backlog
---

## Description

Follow-up of TKT-D3RAB0E. `planSearch` (`internal/dataentry/searchplan.go`)
declines these structured queries, so they still read every row of the named
types through `visibleListByTypes`:

- a sort over several types (the Go path sorts the union);
- `sort:modified` and `sort:id` (virtual keys);
- filters on dates, numbers, booleans, globs, legacy `status`/`priority` types, and `!=` with no value;
- a query that names no type.

Also: a structured search without a limit and without a filter reads up to the
1000-row cap, so `TestQueryBudget_SearchIsSizeIndependent` uses a status filter.
The `_position` gain is postgres-only, since `storetest.Counting` does not
forward `store.PositionQueryer`; a postgres budget test would pin it.

Lower each shape only where the store judges it exactly as `filter.MatchAll` and
`filter.QuerySort` do, and extend `TestSearchPlan_MatchesGoPath`.
