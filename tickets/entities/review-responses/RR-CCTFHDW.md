---
id: RR-CCTFHDW
type: review-response
title: Go path id mode still reads every neighbour
finding: 'In id mode matchRelationFilterMany still runs servedIDsErr over every distinct neighbour although the target is known. Fix: keep only edges whose far end is target.id.'
severity: minor
resolution: relationFilterNeighbors answers an id target from the resolved target without reading the neighbours.
status: addressed
---
