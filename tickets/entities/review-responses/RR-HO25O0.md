---
id: RR-HO25O0
type: review-response
title: Property pushdown changed results on the shared executeQuery path
severity: critical
status: addressed
finding: 'The pushdown removed pushed filters from the Go pass, so store.PropPredicate (string-form comparison) became authoritative for them instead of the metamodel-aware filter.MatchAll. They disagree on typed properties: count!=03 against an integer 3 is a non-match typed and a match as strings; flag!=yes on a boolean errors in Go (excluding the row) and matches in the store. Two of the divergences were WIDENINGS, on a path /_search and scope navigation share.'
resolution: 'The pushdown is now a pure PRE-FILTER: the Go pass still evaluates every filter and stays authoritative, so the store can only remove rows that pass would also remove. A first attempt at this was still wrong -- pushing count=03 NARROWED away a row the typed comparison matches -- so eligibility is now checked against the metamodel: only properties declared string on EVERY named type in the query are pushed, and an unnamed-type query pushes nothing. Not-equal is excluded entirely, since it is the operator where a disagreement widens. Tests cover the typed-property case end to end through the real metamodel.'
---
