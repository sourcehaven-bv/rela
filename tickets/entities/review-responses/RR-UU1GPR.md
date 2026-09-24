---
id: RR-UU1GPR
type: review-response
title: Copy Related when composing onto the ACL query
finding: Appending onto the ACL query's Related relies on the ACL never setting it; the ReadQueryResult is cached.
severity: nit
resolution: 'Plan: copy like Props: append(append([]store.DirectedRelation(nil), gq.Related...), frag.Related...).'
status: addressed
---
