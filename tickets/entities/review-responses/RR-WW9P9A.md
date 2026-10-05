---
id: RR-WW9P9A
type: review-response
title: Post-rename check behaves differently per backend for relation-conferred roles
finding: The post-rename authorization names oldID. On fs and memstore the relations are already re-keyed, so a local role conferred through a relation is lost and the face is denied; on pg and sqlite the outer handle sees the old graph. Both fail closed.
severity: minor
resolution: Documented in the familyRename.inTx godoc. It only applies to a face no earlier check saw.
status: addressed
---
