---
id: RR-QFA08UH
type: review-response
title: Scan guard misses aliased queries
finding: The source guard on scanHeaders callers accepted queries that did not come straight from scopedQuery (aliases, reassignments, req mutation).
severity: significant
resolution: The AST guard now requires plain identifiers, one scopedQuery assignment of q with matching class and req, only Offset or Limit writes, and no req writes. TestScanCallProblem self-tests it.
status: addressed
---
