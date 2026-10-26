---
id: RR-3KCKWD
type: review-response
title: Moderators can release a pending outputs mark, which drops the outputs and the close
finding: 'ReleaseAction (internal/realtime/actions.go:116-146) takes no action name. It releases whatever action is pending on the message once PendingAt + PendingRelease (15 min, realtimelimits/limits.go:104) has passed. It is moderator-gated through message_ops.remove, and the notice is a message. The outputs mark keeps PendingAt = notice time across claims and progress. A mark that waits through a degraded period, or 5 attempts with a 2 min TTL, passes 15 min, and a release then moves it pending -> none. That erases the outbox, records AuditReleased, and leaves the session moved with no outputs and no close. The plan''s claim "No request can ... release ... an outputs action" rests on PrepareMessage and RunAction, not on this path. The lifecycle sweep also warns "a moderator can release it" for these marks (appbuild/realtime_worker.go:560-563) when the type declares actions. Plan change: releaseGateCheck refuses an `outputs` action with 409 `outputs_pending`, or the plan defines a moderator "abandon" that writes done with failed results. Exclude `outputs` from Sweeper.stalePending (sweep.go:217). Add TestReleaseAction_RefusesOutputsMark.'
severity: significant
resolution: releaseGateCheck refuses an outputs action with 409 outputs_pending. Sweeper.stalePending and the sweep warning skip outputs. Added TestReleaseAction_RefusesOutputsMark and TestSweep_SkipsOutputsMark.
status: addressed
---
