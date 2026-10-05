---
id: RR-8RA7WV
type: review-response
title: Identity-scoped edges read the zero face twice
finding: With FromFace empty, edgeSourceType called GetEntityState(id, "") and then anyFaceOf repeated the same read.
severity: minor
resolution: edgeSourceType reads the tail only for a named face and goes straight to anyFaceOf otherwise.
status: addressed
---
