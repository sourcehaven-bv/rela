---
id: RR-HJ855LT
type: review-response
title: Face-subset refusal broader than needed
finding: Compare grant FaceIn with the read scope FaceIn before refusing; normalize FaceIn [""] in lowerWhen.
severity: minor
resolution: 'grantCond decides a grant''s FaceIn against the read scope''s faces: disjoint is false, subset clears FaceIn, anything else refuses. lowerWhen no longer emits a zero-face FaceIn for faced types. TestGrantCond_Faces and the dora fixture pin it.'
status: addressed
---

## Finding

Compare grant FaceIn with the read scope FaceIn before refusing; normalize
FaceIn [""] in lowerWhen.
