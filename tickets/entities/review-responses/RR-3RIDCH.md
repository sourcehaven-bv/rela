---
id: RR-3RIDCH
type: review-response
title: Chained incoming hop into a relation-granted type fails per principal
finding: A hop landing on a type read through a role relation occupies the endpoint's inbound slot; a following incoming hop collides and the request 500s for that principal only.
severity: significant
resolution: 'The refusal now surfaces as HTTP 422 query_scope_unsupported naming the cause instead of a 500, and docs/metamodel.md lists the limit. Test: TestQueryScopeTraversal_UnsupportedForPrincipalIs422. Supporting the conjunction in EndpointPredicate is a store change across backends, tracked as TKT-44PVX2.'
status: addressed
---
