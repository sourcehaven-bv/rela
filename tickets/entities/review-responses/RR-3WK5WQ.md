---
id: RR-3WK5WQ
type: review-response
title: A redelivered child that already settled re-executes its side effect
finding: The plain-task and expansion handlers are guarded by the StartRun compare-and-set, but the child handler executes before SettleChild. neoq redelivers a finished job when its outcome commit fails; SettleChild's idempotence prevents a double count but not a double mail.
severity: significant
resolution: 'StartChild also returns false for a subject that already settled, so a redelivery after settlement does not execute. Pinned by TestForEach_RedeliveredChildDoesNotExecuteAgain. Not covered: two deliveries of the same child running at the same moment both claim, because a failed attempt must stay claimable for the queue''s retry; neoq delivers a job to one worker at a time, so this needs an outcome-commit failure while the first attempt is still running.'
status: addressed
---
