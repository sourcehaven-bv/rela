---
id: RR-BB1ZW8T
type: review-response
title: Conflict merge on a non-saver erases peers' typing
finding: applyContent uses replaceAll with a body computed from the content sent, so text peers typed during the round trip is deleted.
severity: minor
resolution: 'Merges go in as a change from the text the save was made from (applyChange) and only while synced and online, so peers'' typing since stays. Test: MilkdownEditorCollab ''keeps what was typed since ...''.'
status: addressed
---
