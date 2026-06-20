---
id: RR-EOXQIB
type: review-response
title: RenameEntity wrote no tombstone for the old id — sync ghost entity
finding: RenameEntity re-keys oldID->newID in place (entity.go:393 UPDATE entities SET id=$2 WHERE id=$1), removing oldID from entities with NO tombstone. To an id-keyed sync client a rename IS a delete of oldID + create of newID; the manifest surfaces newID (fresh seq) but never reports oldID is gone, so the client keeps a ghost entity forever. notifyDelete(oldID) at entity.go:436 is an in-process observer call, not a durable tombstone. This is the exact bug class the ticket fixes, via a different removal verb.
severity: critical
resolution: RenameEntity now writes an entity tombstone for oldID inside the rename tx (after the re-key, before commit), so the manifest reports the removal. Regression TestRenameTombstonesOldIdentities asserts the DEC-1 tombstone appears alongside the live DEC-2 row in the manifest.
status: addressed
---
