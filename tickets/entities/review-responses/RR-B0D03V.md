---
id: RR-B0D03V
type: review-response
title: admin.list_entities lists nothing for an undeclared face
finding: The elevated path skipped the declared-face check, so a typo returned an empty list a script cannot tell from no rows.
severity: significant
resolution: listFaces holds the world and the metamodel and resolves the selection for both paths, so both raise the same error. Pinned by the admin cases in TestListEntities_FaceOptionRejectsBadValues.
status: addressed
---
