---
id: RR-28F1X4Y
type: review-response
title: A held field does not clear when the local side takes the remote value
finding: Conflicted rows record the source hash, so an entry stayed until the remote changed even when the local side adopted the remote value.
severity: minor
resolution: The push drops a both_changed entry whose local digest equals the remote digest and moves the base. TestPush_AgreedConflictClears.
status: addressed
---
