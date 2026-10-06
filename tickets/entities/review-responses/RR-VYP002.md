---
id: RR-VYP002
type: review-response
title: 'Format test covered only a missing stamp'
finding: 'The rebuild test removed the format key but never set a different value, and did not close the index on a failed assertion.'
severity: minor
resolution: 'The test now covers a missing and an older value, uses t.Cleanup, and seeds more documents than one clearing pass deletes.'
reason: 'Covers both branches of the check and the clearing loop.'
status: addressed
---
