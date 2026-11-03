---
id: RR-4WUC0Y
type: review-response
title: Bare id of a faced relation target raises with no way to list faces
finding: Relation heads are bare ids, so get_entity(r.to) raises for a faced target and the script cannot discover the face names.
severity: significant
resolution: Added rela.get_faces(id), returning every stored face row sorted by face. The ambiguity error points to it. Docs say when to use it; tested in TestLuaReads_StepFaceAndFaces.
status: addressed
---
