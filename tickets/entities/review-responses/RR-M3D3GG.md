---
id: RR-M3D3GG
type: review-response
title: '[security] Relation PATCH/DELETE did not gate the peer'
finding: handleV1UpdateRelation and handleV1DeleteRelation gated only the path entity. An edge to a peer with no readable face answered differently from an edge to an absent id. That is an existence oracle on the peer.
severity: significant
resolution: Both handlers now call familyReadableOr404 on the peer before touching the edge. TestRelationWrites_HiddenPeerIsTheUniformMiss pins that a hidden peer matches an absent one and that the edge stays.
status: addressed
---
