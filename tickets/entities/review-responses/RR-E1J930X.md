---
id: RR-E1J930X
type: review-response
title: setContentBase leaves lastSeenContent stale
finding: setContentBase moves the merge base but not lastSeenContent. A document that returns to the client's original load body is then dropped by fireContent (value === lastSeenContent) although the store holds a peer's later body.
severity: significant
resolution: 'setContentBase now sets lastSeenContent, so the dedupe follows the room''s base. Test: useAutoSave.conflicts ''drops a save of the body a room base set''.'
status: addressed
---
