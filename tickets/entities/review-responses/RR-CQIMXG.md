---
id: RR-CQIMXG
type: review-response
title: Metamodel snapshot read twice in one operation
finding: handleV1GetRelationType read a.Meta() and later a.State().Meta; gatherListPeers called h.meta() per row.
severity: nit
resolution: Both now capture the metamodel once per operation.
status: addressed
---
