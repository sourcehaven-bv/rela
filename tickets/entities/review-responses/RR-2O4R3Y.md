---
id: RR-2O4R3Y
type: review-response
title: Retried failures and closed-channel skips do not count against MaxSystemPostsPerRound, so one round can make unbounded store calls
finding: 'internal/realtime/trigger.go:878 (the cap checks only rep.Posted+rep.Failed), 902-911 (ErrNotOpen increments Skipped; a non-final failure requeues and returns before Failed++). Each attempt runs validateProps, recordType (a possible RecordTypes store call) and an Append. Scenario: a reconcile after a today()-based rule flips, or after an outage, queues 50k enter effects. A persistent error (for example ErrTypeShapeConflict or configChanged during a reload, or a store outage) makes every one of them fail without counting. The round then makes 50k failing store writes, and repeats that for TriggerRetries rounds. Similarly, under when_closed: skip, every closed channel costs an Append per round without counting. Fix: count every attempted Append (posted, skipped or failed, including non-final failures) against the per-round cap, and requeue the rest untouched as the code already does for overflow.'
severity: minor
resolution: Every attempt counts against MaxSystemPostsPerRound, including retries and closed channel skips, and a failing store stops the round after one attempt.
status: addressed
---
