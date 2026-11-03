---
id: RR-0H0LOU
type: review-response
title: Every change replays every subscription with no round limit
finding: 'Code review: a write to any type replays all subscriptions. A round has no time limit and rounds run back to back.'
severity: significant
resolution: 'A round starts no replay after 10 s (liveRoundBudget); the rest fall back like the cap. Rounds run at most once per 500 ms tick. Skipping subscriptions by changed type was rejected: access (principal_property and relation grants) and include=* responses depend on other types.'
status: addressed
---
