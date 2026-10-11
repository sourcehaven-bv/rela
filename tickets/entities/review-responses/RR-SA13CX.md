---
id: RR-SA13CX
type: review-response
title: pushdownScope invariant not enforced
finding: An empty scope fell through to a nil GraphQuery dereference; GraphQueryer re-asserted with gq
severity: minor
resolution: The scope carries the GraphQueryer; both callers switch on graph explicitly and fail closed with errEmptyScope otherwise.
status: addressed
---
