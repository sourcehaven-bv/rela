---
id: RR-TLQPK6
type: review-response
title: FaceOrderOf copies EntityDef per call
finding: FaceOrderOf takes EntityDef by value and sortFaces builds a rank map per call (cranky review).
severity: nit
reason: Faced types have a handful of faces, so the cost is small. A per-type precomputed rank fits when the resolver takes the metamodel directly in a later PR.
status: deferred
---
