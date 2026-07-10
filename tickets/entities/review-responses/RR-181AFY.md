---
id: RR-181AFY
type: review-response
title: Cascade-deleted relations are never versioned (silent, unrecoverable history loss)
finding: 'The design captures relation deletes via a hook on Manager.DeleteRelation, but the dominant way relations die is Store.DeleteEntity''s cascade: a bulk `DELETE FROM relations WHERE from_id=$1 OR to_id=$1` (pgstore/entity.go ~L317), entirely below the entitymanager. Deleting one hub entity destroys all its edges with zero delete-version rows, and because the rows are gone no sweep tick can backfill them. Fix: capture relation-delete VersionInputs where the cascade happens — Store.DeleteEntity already materializes the affected relations (the `related`/DeletedRelations slice, entity.go ~L307/L352) before the bulk delete, so emit versions from that slice in the same tx, attributed from ctx. Prefer making DeleteResult.DeletedRelations the single source for ALL relation-delete capture (cascade + explicit) so there is one path, not two that drift (cranky L2).'
severity: critical
resolution: 'Design revised: DeleteResult.DeletedRelations is now the single source for ALL relation-delete capture (cascade + explicit), emitting delete-versions with full pre-delete snapshot in the delete tx. See revised ticket ''Synchronous delete'' section.'
status: addressed
---
