---
id: RR-AVGNR9
type: review-response
title: Face-only read grants reported as type-wide read
finding: 'roleGrantsRead counts type@face as a grant on the type, so read: [beleid@concept] showed as read on every beleid.'
severity: significant
resolution: splitFaces decides every verb of a faced type per face; a face-scoped read is now listed under Faces. Covered by TestMapPrincipal_FacedTypeDecidedPerFace.
status: addressed
---
