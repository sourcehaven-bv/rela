---
id: RR-0JQWN6W
type: review-response
title: A parked job branch blocks the other branches' timers
finding: advanceBranches settles whenever Pending(run) is true, so a branch with an in-flight or failed job stops timers elsewhere.
severity: significant
resolution: 'Plan: pendingToken and Pending treat a parked job token as settled, so other branches'' timers fire. Test with a fork of a job branch and a timer branch.'
status: addressed
---
