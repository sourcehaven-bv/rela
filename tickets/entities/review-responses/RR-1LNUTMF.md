---
id: RR-1LNUTMF
type: review-response
title: A failed finish leaves the run stuck
finding: If the header patch or the delete in finish failed, the run stayed stored in a final state with no mark, so nothing ever retried the end of the run.
severity: critical
resolution: Every move now stores a pending mark that stays until the run waits or ends. A failed finish keeps the mark, so Due retries it. TestDue_FailedFinishIsRetried pins it.
status: addressed
---
