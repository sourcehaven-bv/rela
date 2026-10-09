---
id: RR-O378LX
type: review-response
title: No version capture for edges and rows a move deletes
finding: The explicit DeleteRelation of moved originals and the source row delete of migrate_face and rename_face had no synchronous pre-delete capture.
severity: significant
resolution: captureMoves records a delete version for every moved row and every edge whose key changes, for all three callers. Pinned by TestMigrateFace_CapturesWhatTheMoveDeletes.
status: addressed
---
