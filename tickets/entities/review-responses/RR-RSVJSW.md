---
id: RR-RSVJSW
type: review-response
title: Rename tombstone has no undo
finding: BeforeRename tombstones channels on newID before the Tx; a Tx failure leaves them detached, and an entity created in the free()/Tombstone gap loses its first channel.
severity: minor
resolution: BeforeRename returns an undo holding the handle; undoFailedRename restores it when the rename Tx fails unless the new id has stored faces.
status: addressed
---
