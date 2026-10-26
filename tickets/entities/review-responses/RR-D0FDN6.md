---
id: RR-D0FDN6
type: review-response
title: Stale next_due_at from writes under an older config is never repaired; a zero hint means the timer never fires
finding: 'Each post builds step.Due from the snapshot it loaded. A post that loaded the old config and commits after the leader already ran the recompute for the new pointer writes an old-config hint; so does any long-lived non-leader process (rela mcp, desktop, a second rela-server) that has not reloaded yet. If the old config had no timer for that state the hint is 0, the row is never listed by ListSessionsDue, and the timer never fires until the next reload or leader change. If the old period was longer it fires late. This breaks the ''never later than the true due time'' invariant (plan line 536). Plan change: add a periodic full recompute (for example every realtime.TriggerReconcile with the existing jitter) and run the recompute a second time one TimerCeiling after a pointer change. Add TestTimers_RecomputeRepairsStaleHint (store a zero hint on an open channel with a due timer, assert the periodic pass fixes and fires it).'
severity: significant
resolution: 'Plan updated: periodic recompute every 10m (jittered) plus a second pass one TimerCeiling after a config change; invariant restated; AC 11 TestTimers_RecomputeRepairsStaleHint added.'
status: addressed
---
