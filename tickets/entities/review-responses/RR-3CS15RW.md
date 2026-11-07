---
id: RR-3CS15RW
type: review-response
title: WithFlowRuns writes into the published snapshot
finding: validate on a shallow copy wrote the shared Types map, compiled patterns and Comments.On.
severity: significant
resolution: cloneForValidate copies Types (with each validations slice), Transforms and Comments before validate. Pinned by TestWithFlowRuns_ConcurrentReaders under -race.
status: addressed
---
