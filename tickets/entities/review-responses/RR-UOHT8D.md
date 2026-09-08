---
id: RR-UOHT8D
type: review-response
title: '[security] PATCH response for a faced address returned the union of every face''s edges'
finding: '[security] The PATCH response built its relations map with the bare-id reader, which returns every face''s content-scoped edges, so a PATCH to the published face answered with the draft''s links beside it — the mixed-face serve servedFaceEdges exists to prevent, on a body the SPA feeds into its relation editor.'
severity: minor
resolution: 'writeHandler gained a faceEdges closure bound to servedFaceEdges (production and test wiring alike) and the PATCH response is serialized through forWireScoped with the face''s own edges. Pinned in TestFacedAddress_PatchWritesTheNamedFace: a draft-tailed `cites` edge must not appear in the published face''s PATCH response.'
status: addressed
---
