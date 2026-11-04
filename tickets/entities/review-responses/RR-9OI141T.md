---
id: RR-9OI141T
type: review-response
title: Peer-side code on an incoming wrapper names the wrong side
finding: collectEdgeWarnings always used target_type_not_allowed for the peer. On an inverse (incoming) wrapper the peer is the edge source so the error code must be source_type_not_allowed or clients misread which end is wrong. The API reference must state that the code names the canonical side.
severity: minor
resolution: Plan step 4 picks the code by canonical side via the metamodel code constants; API reference documents it.
status: addressed
---
