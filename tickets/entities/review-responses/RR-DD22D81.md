---
id: RR-DD22D81
type: review-response
title: Boolean related constraints derive no index
finding: queryplan derives no index for them.
severity: minor
reason: Needs a new index shape with its own EXPLAIN test; documented in traversalIndexTarget and the ACL guide.
status: deferred
---
