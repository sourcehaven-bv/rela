---
id: RR-V3Q5NA
type: review-response
title: Rename/delete hook must be owner-free and is non-transactional
finding: The hook runs under the acting principal (maybe system:*) which the owner check would refuse; pg writes are outside the store Tx.
severity: nit
resolution: 'Plan: the AliasRewriter methods bypass owner checks and are not reachable from HTTP. Documented that the pile rewrite cannot roll back with a store Tx, as with comments.'
status: addressed
---
