---
id: RR-YYTC1K
type: review-response
title: Form decision watch shows a stale toast for a decision the user has already abandoned by switching type
finding: useSessionDecisions.ts:146-155. After a form decision the user switches the composer to another type, for example to write a comment. If the session then moves, the watch clears the decision and toasts "The session moved on. Choose again." while the user is writing something else. Deviation 6 causes this, because the composer exposes no selection event. Either expose the selected type from RlChannelComposer (one emit) and drop the decision on change, or suppress the toast when the composer's current type is not formDecision.messageType.
severity: minor
resolution: 'Fixed by the same select event: switching type drops the form decision silently, so a later session move shows no stale toast (useSessionDecisions test "switching the composer to another type ...").'
status: addressed
---
