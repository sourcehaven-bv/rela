---
id: RR-76GV4W9
type: review-response
title: The claim that a run is visited once per tick is false
finding: A run backed off in round 1 to now+30s can come due again in a later round of the same tick when rounds are slow, so the runTick and DueRound comments overstate the guarantee.
severity: minor
resolution: Both comments now say only that each round goes on after the last run of the previous one. The compare-and-set keeps a second visit safe.
status: addressed
---
