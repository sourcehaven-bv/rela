---
id: RR-QAC4HXL
type: review-response
title: The reach is keyed by id, not by face
finding: With a faced row type, an edge landing on a face the reader cannot read still admits another readable face of the same id. The list tab has the same behaviour today.
severity: minor
resolution: 'Fixed for #2366: anchorReach now gates incoming edges with visibleReader.readableRelations (one batched endpoint gate), so a row is reached only over an edge whose tail face (Relation.FromFace) the reader may read. Covers list tabs and dashboard cards; pinned by TestPageScope_IncomingReachIsGatedOnTheTailFace.'
reason: Pre-existing in resolvePageScope for list tabs; fixing it belongs with the page scope, not this ticket. Card counts reuse the same rule, so both stay consistent.
status: addressed
---
