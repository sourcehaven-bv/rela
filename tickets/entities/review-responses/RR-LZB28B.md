---
id: RR-LZB28B
type: review-response
title: Retention on a never-closed channel passes because AddTo(zero ClosedAt) is in the past
finding: B3 step 11 computes until = Retention.AddTo(ClosedAt). Reopen clears ClosedAt (ApplyTransition), and delete's natural source is any state, so an open channel gets until = year 0007 and is deleted despite D4's "close first". Special-case a zero ClosedAt in lifecycleCheck and add an open-channel row to TestLifecycle_DeleteRetention.
severity: minor
resolution: A zero ClosedAt under retention refuses delete with retention_active ("close first"). TestLifecycle_DeleteRetention gains an open-channel row.
status: addressed
---
