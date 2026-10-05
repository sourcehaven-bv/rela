---
id: RR-YGWX0O
type: review-response
title: Named deleted face in a denied world answered 200 while a live one answered 404
finding: With history:read, a named face in a denied world returned 200 world_face_absent when deleted and 404 when live, which tells the two apart.
severity: minor
resolution: A named face in a denied world is now the uniform 404 whether live or deleted; a bare id keeps world_face_absent. Pinned by TestFacedHistory_DeniedWorldHidesNamedFaces.
status: addressed
---
