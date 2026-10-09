---
id: RR-Y28CSM
type: review-response
title: 'Code: a save is lost when it lands as a job finishes'
finding: The idempotency key frees only after the queue records completion, so a save in that window got ErrDuplicateJob and no run.
severity: significant
resolution: Runs record the handled token; a collapsed enqueue starts a bounded follow-up that re-enqueues until the token is handled or a new job is queued. TestAutomationJobs_CollapsedSaveIsFollowedUp (mutation-checked).
status: addressed
---
