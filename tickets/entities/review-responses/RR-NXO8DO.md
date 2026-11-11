---
id: RR-NXO8DO
type: review-response
title: Missing negative tests for the redirect
finding: 'Tests covered the redirect and the hidden successor but not: a live id of another type under the route, a live id with a bogus face, a dangling successor, the lookup-failure path, and body parity of the 422 in a ranking world.'
severity: minor
resolution: Added TestRetiredRedirect_Misses (other type, undeclared face, deleted successor, on both the entity GET and the view), TestRetiredRedirect_LookupFailureIsAMiss and TestRetiredRedirect_ViewsInAWorldHiddenSuccessor (422 body parity in a world).
status: addressed
---
