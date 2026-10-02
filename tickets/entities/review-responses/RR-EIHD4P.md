---
id: RR-EIHD4P
type: review-response
title: show_entity lists relations from every face
finding: buildStoreRelations queried outgoing edges without FromFace, so show_entity presented another face's content-scoped links as the served face's (BUG-VFHUWO shape).
severity: critical
resolution: Outgoing edges are filtered to the served face (FromFace), as the data-entry GET does. TestBuildStoreRelations_OutgoingEdgesOfTheServedFaceOnly, mutation-checked.
status: addressed
---
