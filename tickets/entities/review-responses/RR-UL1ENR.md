---
id: RR-UL1ENR
type: review-response
title: Missing conformance cases
finding: PurgeMarked, DeleteFace, write during fs build, and stale-hash rows were not tested.
severity: nit
resolution: 'Covered: conformance SoftDeleteHidesAndUnmarkRestores (PurgeMarked), FacesIndexedSeparately (DeleteFace), fs TestMentions_WriteDuringBuildIsKept, pg stale-hash heal test. Every conformance case runs Cold and Warm.'
status: addressed
---
