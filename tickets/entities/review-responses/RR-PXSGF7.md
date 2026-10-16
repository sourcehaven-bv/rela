---
id: RR-PXSGF7
type: review-response
title: Delete racing a rename strands channels
finding: Delete guard tombstones X, a rename X to Y commits, the delete Tx fails not-found, familyStillStored(X) is false so no undo, and the rename Rekey finds nothing. Y loses its channels.
severity: minor
resolution: A delete that fails not-found with its anchor gone logs the stranded tombstone handle (id, face, handle) at warn.
status: addressed
---
