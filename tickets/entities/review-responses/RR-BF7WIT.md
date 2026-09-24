---
id: RR-BF7WIT
type: review-response
title: Fallback could recurse through store helpers
finding: store.CountMatched/GraphQueryHeaders on s type-assert back to the same methods.
severity: minor
resolution: Fallbacks call graphquerynaive directly and build headers from its rows.
status: addressed
---
