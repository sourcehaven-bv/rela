---
id: RR-7QQPYC
type: review-response
title: Nil edge row was skipped
finding: Skipping a nil row lowers a count and can invent a min violation.
severity: minor
resolution: A nil row now fails the check; TestCheckCardinality_NilEdgeAborts.
status: addressed
---
