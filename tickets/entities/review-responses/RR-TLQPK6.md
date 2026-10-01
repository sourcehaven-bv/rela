---
id: RR-TLQPK6
type: review-response
title: FaceOrderOf copies EntityDef per call
finding: FaceOrderOf takes EntityDef by value and sortFaces builds a rank map per call (cranky review).
severity: nit
resolution: FaceOrderOf takes the metamodel and reads the recorded face order through the map, returning a clone, so no EntityDef is copied and no rank map is built per call.
reason: Faced types have a handful of faces, so the cost is small. A per-type precomputed rank fits when the resolver takes the metamodel directly in a later PR.
status: addressed
---
