---
id: RR-GNBENK
type: review-response
title: 'Security: deleted row listed when its face is live again but hidden'
finding: deletedRowGate.admits admitted a deleted tagged row whenever any sibling face was readable. That included the case where the same face had been recreated and the new row was hidden. The HTTP history endpoint treats a live named face as a live read and returns 404. The script could learn that the hidden face's lineage carried the tag.
severity: minor
resolution: admits now requires the recreated face itself to be readable when the deleted face lives again. TestTaggedEntities_DeletedFaceLivingAgain covers the hidden and the readable case.
status: addressed
---
