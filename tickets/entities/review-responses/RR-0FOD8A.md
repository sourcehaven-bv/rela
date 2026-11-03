---
id: RR-0FOD8A
type: review-response
title: Round cap counts only successes, so a failing store gets every pending effect each tick
finding: 'internal/realtime/trigger.go:868-888. The cap check is `r.rep.Posted+r.rep.Failed >= MaxSystemPostsPerRound`. Retryable failures (requeued), ErrNotOpen skips and cooldown deferrals do not count. With 5k pending effects and a store that rejects Append, each 500 ms tick calls recordType + Append 5k times, then 3 ticks later logs 5k ErrorContext lines and marks all 5k seen (see the retry finding). Fix: count every Append attempt against the cap (attempts, not outcomes), and stop the round early once one store error suggests an outage (or once ctx is done).'
severity: significant
resolution: The 64 cap now counts every Append attempt, including skips and failures. A transient store error or a canceled context stops the round, and the rest stay queued.
status: addressed
---
