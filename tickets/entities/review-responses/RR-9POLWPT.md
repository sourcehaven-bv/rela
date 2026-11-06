---
id: RR-9POLWPT
type: review-response
title: flow would pull predicate into the channel store
finding: Aliasing channels.Session to flow.Instance makes channels import flow. With GuardHolds in flow, flow imports predicate and predicatefns, so every channel backend compiles the guard engine.
severity: significant
resolution: 'Addressed in plan: GuardFacts and GuardHolds stay in realtimecfg. flow.Decide takes the guard as a callback and reminder counters come back as map[string]int64. flow depends only on realtimelimits.'
status: addressed
---
