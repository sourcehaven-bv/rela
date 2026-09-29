---
id: RR-1A6PUH
type: review-response
title: Self-edge rule and most incoming surfaces were untested
finding: The self-edge clause in Neighbors/NeighborsForPage had no test, and the new dataentry test covered only outgoing edges on the bare-id surfaces.
severity: significant
resolution: Added TestNeighbors_ContentSelfEdge (both readers agree in all three directions), the incoming relation filter test, TestIncomingOwnedAtZero (self-edges and faced/faceless sources), and route assertions for /relations and /relations/{type}?direction=incoming. The recursive _views traverse shares ownedEdges with the flat path that is tested; the create/restore/upload responses reuse edgesOwnedBy, which the export and list tests exercise.
status: addressed
---
