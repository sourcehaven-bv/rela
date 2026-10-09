---
id: RR-2H3J4N
type: review-response
title: Face move re-tails identity edges onto the new face
finding: A move from the implicit face re-created identity-scoped zero-tail edges on the destination face. Readers still find them but a later delete of that face removes them and the manager refuses to write an identity edge with a face.
severity: critical
resolution: DeleteFace on a non-last implicit face now keeps zero-tail edges on all four backends (storetest ZeroFaceDeleteWithSiblingsKeepsZeroTailEdges). applyFaceMove carries only content-scoped edges from the implicit face using the scope from the to-projection or the live metamodel.
status: addressed
---
