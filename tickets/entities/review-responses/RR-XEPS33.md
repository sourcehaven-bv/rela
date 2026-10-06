---
id: RR-XEPS33
type: review-response
title: Incoming relation filter checks ownership at the default face, not the served face
finding: '[security] matchRelationFilterMany read neighbor headers with no World, so an incoming edge''s source was resolved at its default state rather than the face the request''s world serves, and candidates passed only the per-id row gate (PermitsReadMany) with no faceReadable check. A published-only reader filtering on cited-by could learn which features the draft cites.'
severity: significant
resolution: 'Neighbor headers are now read with World: worldScopeFrom(ctx) and skipped unless faceReadable(ctx, type, face) holds, before the title comparison and the ownership check. TestContentEdges_IncomingRelationFilterMatchesTheServedFace covers both readers in the published world and a draft-selecting world; it fails when either the World or the face gate is removed.'
status: addressed
---
