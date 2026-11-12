---
id: RR-E9EU74K
type: review-response
title: Leader overlap is longer than the comment says
finding: check runs only between ticks and a tick can take 4 rounds of up to 1 minute, so two leaders can overlap for more than the takeover interval.
severity: minor
resolution: 'The flow_worker.go comment and the plan now state the bound: a takeover interval plus one tick. Compare-and-set and the create claim keep the overlap safe.'
status: addressed
---
