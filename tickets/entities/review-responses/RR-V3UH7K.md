---
id: RR-V3UH7K
type: review-response
title: Some listings still sort face names
finding: entitymanager sortedFaceNames, docs facesOf and seed, and worlds.Compiled.Names sort instead of using declaration order (cranky review).
severity: nit
resolution: entitymanager face errors use FaceOrderOf; worlds.Compiled.Names and declaredFaces follow WorldOrderOf/FaceOrderOf; the docs faces table and declaredWorlds use declaration order.
reason: Error-message and listing order moves to FaceOrderOf and WorldOrderOf in the later stage 3 PRs that touch those files.
status: addressed
---
