---
id: RR-NVVALGM
type: review-response
title: Test gaps against AC 3 to AC 7
finding: AC 3 lacks timer move, plan and legacy cases; AC 4 realtime test claims a non-poster case it does not test; AC 5 handler test has no baseline compare; AC 6 route budget does not assert at most one ref resolve; AC 7 benchmark fill is opt-in so recorded numbers must state the fill.
severity: minor
resolution: 'Added TestInbox_WaitingAfterTimerMove (AC 3 timer move: no cause, no read) and TestInbox_DecisionInputsWithoutPost (AC 4 non-poster); fixed the TestInbox_DecisionInputs doc. The AC 5 handler test now compares the page byte for byte before and after the hidden rows. The AC 6 route test pins one extra entity read for all refs against the summary. The AC 7 numbers in IMPL-SJYXHEE state the fill (700 file, 50 postgres).'
status: addressed
---
