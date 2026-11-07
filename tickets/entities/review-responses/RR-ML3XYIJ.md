---
id: RR-ML3XYIJ
type: review-response
title: Undo to a saved body is never saved
finding: useBodyCollab skips any body in the room's saved-body list. After saves F1 and F2, undo back to F1 schedules nothing, so the store keeps F2 while every editor shows F1. A pending save of F1+'a' also fires after the user deletes the 'a'.
severity: critical
resolution: 'Saved-body tracking is now the room''s latest save only (isCurrentBase on base.content/base.sent), the hash list is gone. setContentBase also moves lastSeenContent, so an undo to any earlier body is sent. Tests: useBodyCollab ''saves an undo back to an earlier saved body'' and the wired useBodyCollab.autosave tests.'
status: addressed
---
