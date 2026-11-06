---
id: RR-1DG0W8D
type: review-response
title: A clean 412 merge never reaches the room
finding: A clean merge or an absorbed body is reported to the page as 'stored', which a room drops. onContentSaved then moves the room base to the merged body and the next room save, which lacks the outside text, overwrites it without a 412.
severity: critical
resolution: 'applyServerContent reports ''merged'' with the text the save was made from, and in a room EntityDetail calls mergedContent(from, to), which applies the change through MilkdownEditor.applyChange (three-way mergeText against the live document). Test: useBodyCollab.autosave ''puts a clean merge ... into the room as a change''.'
status: addressed
---
