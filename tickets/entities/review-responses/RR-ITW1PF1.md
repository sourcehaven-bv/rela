---
id: RR-ITW1PF1
type: review-response
title: flowcfg refuses two branches that fork to the same states
finding: Role or guard alternatives forking to the same targets were checked as two forks and refused with a misleading message.
severity: minor
resolution: Forks are identified by state and sorted targets (forkKey) and checked once. TestParse_ForkAlternatives.
status: addressed
---
