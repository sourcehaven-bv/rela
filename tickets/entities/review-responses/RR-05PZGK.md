---
id: RR-05PZGK
type: review-response
title: Nesting suite misses Limit and Tx cases
finding: GraphQuery with Limit (the single-statement branch) and ListRelations/GraphQuery inside a Tx were not covered.
severity: minor
resolution: Added GraphQueryLimit and an InsideTx subtest for every store case.
status: addressed
---
