---
id: RR-UAL3GB
type: review-response
title: Counting budget is meaningful only on SQL backends
finding: graphquerynaive MatchingIDs does per-candidate lookups below the Counting wrapper, so AC5 measures pg/sqlite only. The plan must say so.
severity: minor
resolution: TestQueryBudget_TraversalScopeIsSizeIndependent pins 6 reads at 10 and 50 rows with one MatchingIDs. It runs on memstore, where MatchingIDs is one counted call; per-candidate work below the wrapper is not measured, so the pg EXPLAIN test covers the SQL shape.
status: addressed
---
