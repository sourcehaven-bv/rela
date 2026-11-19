---
id: RR-B7IKG5Y
type: review-response
title: ACL gate reads past role edges under as_of
finding: LocalRolePredicate reads the shadowed relations CTE, so a revoked role grants reads at an earlier instant.
severity: critical
resolution: The ACL gate (HasInbound/HasOutbound and Any branches) renders through buildGatePredicateSQL on both database backends. Its edges and endpoint rows read the live tables past the snapshot CTEs, so the past is read with today's ACL and lost access fails closed. Pinned by storetest RunInstantTests/ACLGateReadsTodaysEdges.
status: addressed
---
