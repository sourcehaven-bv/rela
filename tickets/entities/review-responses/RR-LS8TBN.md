---
id: RR-LS8TBN
type: review-response
title: Reminder sub-cap does not stop a reminder backlog from delaying deadlines
finding: 'Plan Approach 6 (lines 584-591) and AC 12 TestTimers_ReminderCap (lines 253-255) claim that a reminder flood cannot hold back a deadline because fires go on past the 32-reminder cap. fireDue lists only min(64+1+backoff, 256) rows ordered by next_due_at (internal/realtime/timers.go:500-501). With more than about 65 overdue reminder slots sorted ahead of a timer, the timer is not in the listing, so it waits until the backlog drains. The 32 sub-cap makes that drain slower (32 per round instead of 64), so it delays exactly the case it is meant to protect. It helps only when the timer already sits inside the listed window. Plan change: drop the claim. Either remove the sub-cap and share the 64 writes, or keep it and document the bound: a deadline can run up to ceil(backlog/32) floor rounds late. Pin that bound in TestTimers_ReminderCap with more than 65 due reminders listed ahead of the timer. A second ListSessionsDue for timers only is the alternative if the claim must hold, at the cost of one more query per round.'
severity: significant
resolution: 'Addressed: sub-cap dropped. All writes share the 64-write cap, and the bound is documented: a deadline behind B due reminders is at most ceil(B/64) floor rounds late. TestTimers_ReminderBacklogBound pins it with 200.'
status: addressed
---
