---
id: RR-92N09RX
type: review-response
title: Reported error depends on map order
finding: checkRelationEdges ranged over the desired map, so with two failing wrappers the code was random.
severity: minor
resolution: Body keys are walked in sorted order.
status: addressed
---
