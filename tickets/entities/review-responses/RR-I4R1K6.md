---
id: RR-I4R1K6
type: review-response
title: Failed-rename restore races a create at the new id
finding: undoFailedRename checks familyStillStored(newID) and then restores; a create committed in between gets the earlier channels attached.
severity: minor
status: open
---
