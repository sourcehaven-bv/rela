---
id: RR-A70TGL
type: review-response
title: Cascade denial judged hidden edges by family, not by tail face
finding: '[security] hiddenFromCaller counted an edge as visible when any face of the far entity was readable. The read path requires the tail face of a content-scoped edge to be readable (RR-2IK76Z), so a denial over POL-a@draft named the hidden draft face and the relation type to a caller who reads only POL-a@published.'
severity: significant
resolution: edgeVisibility (hiddenedges.go) applies the read path's head/tail rule. Pinned by TestCascadeDelete_EdgeFromHiddenFaceNamesNothing.
status: addressed
---
