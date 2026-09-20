---
id: RR-1M0PZ7
type: review-response
title: SQLite SQL ordering diverged from the Go evaluator for floats, lists and objects
finding: The pushed sort key is the value's SQL text form while graphquerynaive uses Go's fmt; a map or a 1e21 float ordered differently, so a list reordered depending on which path served it. The differential fixture missed both.
severity: critical
resolution: Every ordered read first runs a one-row probe for sort-key values of JSON type real, array or object; a hit declines to graphquerynaive. Fixture extended with a map, 1e21 and a 13-digit integer; a second test pins that exact scalars are still ordered in SQL.
status: addressed
---
