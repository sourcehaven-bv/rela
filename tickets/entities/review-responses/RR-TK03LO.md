---
id: RR-TK03LO
type: review-response
title: Faced targets asymmetric with forward rendering
finding: Forward links resolve only implicit-face rows; backlinks by bare id would list sources on a faced B.
severity: minor
resolution: Forward mentions now resolve through the world-aware batch resolver used for relation neighbours (worldNeighbors.resolveHeads), so faced targets link to the face the reader's world serves. Backlinks key on the bare id and show on any face of B. Tests added for both directions and for a hidden served face.
status: addressed
---
