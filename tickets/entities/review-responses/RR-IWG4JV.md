---
id: RR-IWG4JV
type: review-response
title: Failed-rename undo logs no handle
finding: When familyStillStored errors in undoFailedRename, the channels stay detached and the log has no tombstone handle.
severity: nit
resolution: Every undoFailedRename case that cannot tell (lookup error, failed Restore, failed re-tombstone) logs the tombstone handle at error.
status: addressed
---
