---
id: RR-W17SU9
type: review-response
title: Restore from tombstone is not covered by the closed-channel decision
finding: 'testSessionScheduleSurvivesMoves keeps NextDueAt across Tombstone/Untombstone, and the due query skips tombstoned rows, but timerDue only knows ReopenedAt. A channel restored a week after its entity was deleted fires every overdue timer in the first round after the restore. That may be right or wrong, but the plan does not decide it. Plan change: state the rule (treat the tombstoned span like closed, which needs the restore time as a reopen anchor, or fire at once) in Approach 4 and the guide, and add a TestTimers row.'
severity: minor
resolution: 'Plan updated: restore of an open tombstoned channel is a resume: Untombstone takes a ResumeSchedule callback; tombstoned span treated like closed; AC 6 and AC 11 TestTimers_RestoreFromTombstone added.'
status: addressed
---
