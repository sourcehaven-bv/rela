---
id: RR-2H22OL
type: review-response
title: Auto-close re-closes a reopened channel within one minute
finding: channels.go:426 StateSince returns CreatedAt for an open channel, and ApplyTransition clears ClosedAt on reopen. So sweep.go:189 sees CreatedAt+after <= now and closes every reopened channel of an auto-close type on the next pass. Add an opened_at/reopened_at time (this PR already has a migration) and use it for StateOpen in StateSince and ListByLifecycle.
severity: critical
resolution: A stored reopened_at makes StateSince count from the reopen, so auto-close restarts from it; TestSweep_ReopenRestartsAutoClose covers it.
status: addressed
---
