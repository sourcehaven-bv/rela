---
id: RR-XJX65R3
type: review-response
title: Rows re-fold when the descriptor changes
finding: toStatus follows the current descriptor, so a status change in realtime.yaml, a state arriving during reload, or a late descriptor re-folds shown rows; the fixed fact claim overstates.
severity: minor
resolution: 'Plan: claim reworded; noticeView test without a session; panel test that a descriptor change re-folds with the view kept.'
status: addressed
---
