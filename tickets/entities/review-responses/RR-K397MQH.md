---
id: RR-K397MQH
type: review-response
title: Prime each page on the base ctx so the two primes compose in either order
finding: 'Design review: TKT-YLG695 primes at handler level. If the iterator''s prime is chained through a ctx the caller never sees, or the caller''s prime is ignored, one of the two does double work.'
severity: minor
resolution: 'Each page is primed on the iterator''s base ctx. PrimeLocalRoles skips ids already primed for the request and the traversal memo skips rows it holds, so a pre-primed ctx costs nothing. TestQueryBudget_PushdownOnPrimedCtxAddsNoPrimeReads: one read on a primed ctx.'
status: addressed
---
