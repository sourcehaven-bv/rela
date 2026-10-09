---
id: BUG-QEC1XJ
type: bug
title: Face move loses edges on a failed carry and re-tails identity edges
description: 'applyFaceMove deleted the source face before re-creating its outgoing edges, so on fs/mem a failed carry could not be recovered by a re-run (GitHub #1628). It also re-created identity-scoped zero-tail edges on the destination face, where a later delete of that face removes them.'
priority: low
effort: xs
why1: applyFaceMove deleted the source face first and then re-created its outgoing edges from DeleteResult.DeletedRelations. A CreateRelation failure after the delete left the remaining edges in no row.
why2: fs and mem serialize a store.Tx but do not roll it back. The delete stays applied when a later write in the same move fails.
why3: Recovery on fs/mem is a re-run and a re-run finds rows by scanning. Once the source row was gone nothing pointed the re-run at the missing edges.
why4: The idempotency argument in applyFaceMove covered the create/delete pair of the row but not the edge carry added later. The carry was written to read what the delete reported.
why5: Crash recovery is reasoned about per step in prose and only the row duplication case had a fault-injection test.
prevention: Carry the edges before deleting the source so every intermediate state still holds the source row. TestMigrateFace_RerunRecoversAFailedEdgeCarry injects a carry failure and checks that a re-run restores every edge.
started: "2026-10-09"
completed: "2026-10-09"
status: done
---

## Description

`applyFaceMove` (internal/datamigration/steps.go) creates the destination row,
deletes the source face, then re-creates the source's outgoing edges on the
destination face from `DeleteResult.DeletedRelations`.

On fs/mem there is no rollback. If a `CreateRelation` fails after the delete,
the remaining edges exist nowhere and a re-run cannot find them: the source row
is gone, so the scan does not pick it up again.

Reported as GitHub #1628 (CONTROL-5-33).

Review found a second defect in the same function. A move from the implicit face
re-created every zero-tail edge on the destination face, including
identity-scoped edges, which belong to the entity. A later delete of that face
then removes them, and the manager refuses to update an identity edge that
carries a face. The cause is that `DeleteFace` on the implicit face removed the
identity edges too, so the move had to re-create them somewhere.

## Reproduction

1. A face move of a row with two outgoing edges on memstore.
2. Fail the second `CreateRelation`.
3. Re-run the move: the second edge is never restored.
