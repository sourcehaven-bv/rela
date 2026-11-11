---
id: RR-EV68J46
type: review-response
title: taskStates adds header groups for branch states
finding: A header never names a branch state, so its group only added dead predicates to the task query.
severity: minor
resolution: Branch states no longer add a group; their join does.
status: addressed
---
