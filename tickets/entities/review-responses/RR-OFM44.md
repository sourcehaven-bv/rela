---
id: RR-OFM44
type: review-response
title: __legend and __hub_N identifiers collide if metamodel defines an entity with that name
finding: The reserved identifiers are hard-coded with no collision detection. A user metamodel with an entity literally named `__legend` (improbable but not forbidden by schema validation) would silently overwrite. Low-probability, cheap to defend against (check for collision, adjust prefix) or at minimum document the reservation.
severity: nit
resolution: Extracted the reserved identifiers to named constants `legendNodeID = "__legend"` and `hubIDPrefix = "__hub_"` with a comment documenting the reservation. The full collision-detection approach is kept out of this PR — a user metamodel literally naming an entity `__legend` would silently overwrite, but the probability is negligible and the fix would add complexity disproportionate to the risk.
status: addressed
---
