---
id: RR-POYCA
type: review-response
title: Plan says DelayIfStillRunning but acceptance criteria say skip
finding: The plan references robfig/cron's DelayIfStillRunning wrapper (line 86), but acceptance criteria require 'skip if previous run still active'. DelayIfStillRunning queues the next run to execute after the current one finishes — it does NOT skip. The cron library does provide SkipIfStillRunning which matches the intended behavior. Using the wrong wrapper would cause tasks to pile up instead of being skipped.
severity: significant
resolution: Updated plan to use SkipIfStillRunning instead of DelayIfStillRunning
status: addressed
---
