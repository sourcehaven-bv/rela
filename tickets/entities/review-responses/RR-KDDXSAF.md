---
id: RR-KDDXSAF
type: review-response
title: No cost budget test for the target lookup
finding: collection-reads.md requires a storetest.Counting budget test for a new read on a list path. resolveRelationTargets adds one ResolveIDsErr per request; pin it at 10 and 50 rows and check the pushed relation filter with AssertScaleFree.
severity: significant
resolution: TestQueryBudget_ListRelationFilterByID pins the same calls at 10 and 50 rows and scale-free rows over 100 and 1,000 padding tasks.
status: addressed
---
