---
id: RR-LWMY4Q
type: review-response
title: A queued run that is never reported hangs the whole scheduler, not just its task
finding: 'enqueueTask waits on a completion channel with only its ctx as an escape, and runDueTasks executes sequentially in the ticker goroutine — so one unreported run stops every other task for the life of the process. Two paths reached it: runTaskJob returned early on an empty script without calling reportInFlight, and jobs.dispatch drops a job WITHOUT invoking the handler when its deadline has passed or its retry budget is spent. A durable backend redelivering a row whose Retries was already incremented (crash mid-run, worker killed by the queue''s own handler timeout) hits the latter on a RetryNever job — which is every task the scheduler submits. Nothing in the seam can report those drops, so the waiter could not be fixed from the queue side alone.'
severity: critical
resolution: 'reportInFlight is now deferred with a named return so it fires on every exit path of runTaskJob, making it structural rather than an obligation each future early return must remember. The submitter additionally bounds its own wait with taskResultTimeout (20m, above the queue''s 15m per-handler cap) so a dropped job surfaces as a failure that advances the retry ladder instead of a silent hang. Confirmed empirically before the fix: a handler returning without reporting blocked the submitter for the full ctx duration and replaced the real error with context deadline exceeded.'
status: addressed
---
