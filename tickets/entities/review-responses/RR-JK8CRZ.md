---
id: RR-JK8CRZ
type: review-response
title: Incoming relation write gate judged faced peers by the path entity
finding: '[security] relationSourceEntity fell back to the path entity when the source had no row at the edge''s tail. A faced peer has no zero-face row, so a new incoming edge, an identity-scoped edge and a tail read fault all evaluated creatable/removable/meta-writable against the TO side, which can be looser than the source''s policy. Raised by both the security and the cranky reviewer.'
severity: significant
resolution: relationSources now returns the tail row, else every row of the peer's family, and relationOpDenial/relationMetaDenial deny if any face denies (family rule). A family read fault answers 500. Only a peer with no stored row keeps the path fallback, since the manager refuses a missing endpoint. TestRelationSources_ReadsTheTailFace pins the four cases and the read fault.
status: addressed
---
