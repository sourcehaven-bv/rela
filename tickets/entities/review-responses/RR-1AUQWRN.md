---
id: RR-1AUQWRN
type: review-response
title: The join poll undercuts a failing branch's backoff
finding: The poll capped a forked run's due time at 5 minutes even while a live branch had a pending mark, so the branch was retried every 5 minutes whatever its backoff.
severity: minor
resolution: joinDue gives no join due time while a live branch carries a pending mark. TestJoinWhen_NoPollWhileABranchIsPending.
status: addressed
---
