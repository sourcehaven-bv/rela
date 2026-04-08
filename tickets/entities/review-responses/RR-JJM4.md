---
id: RR-JJM4
type: review-response
title: filters data model can't hold operators (Record<string,string>)
finding: 'Current filters ref is Record<string,string> with no operator slot. AC6 (filter[due_date][lte]=$today round-trip) is impossible: operator gets dropped on read, queryParams hardcodes eq on write, browser back becomes a URL the user never visited. Silent data corruption masquerading as a sync feature.'
severity: critical
resolution: 'New FilterState type: Record<string, {value, op?}>. Operators round-trip via parseFilterQueryParams and buildQueryWithFilters. EntityList and FilterBar refs reshape to match.'
status: addressed
---
