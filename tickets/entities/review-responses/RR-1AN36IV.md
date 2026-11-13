---
id: RR-1AN36IV
type: review-response
title: A join that holds can be stranded after a failed rejoin write
finding: 'An arrival and the join firing are two writes. put only made a partly arrived when: join due, so a crash or failed write between the two left a run whose join holds (all arrived, final arrival, any) with no due time: stranded for good.'
severity: significant
resolution: 'put now takes the join''s due time from joinDue: a join that holds by its shape is due 30s after the write, a partly arrived when: join after JoinRecheck. TestJoin_FiresOnALaterRoundAfterAFailedRejoin drops the rejoin write and a later round fires the join.'
status: addressed
---
