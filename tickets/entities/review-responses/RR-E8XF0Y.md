---
id: RR-E8XF0Y
type: review-response
title: Four godoc comments displaced onto the wrong declaration
finding: New functions were spliced between an existing doc comment and its declaration in pgstore/graphquery.go, pgstore/world.go, store/graphquery.go and storetest/graphquery.go, so godoc showed one function's doc on another.
severity: minor
resolution: Each displaced doc moved back onto its own declaration; verified with grep that GraphCount, worldSQL, GraphBranch and mustRel are immediately preceded by their comments.
status: addressed
---
