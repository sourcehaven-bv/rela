---
id: RR-QOSJZ5
type: review-response
title: No test pins the 'a failure must not touch state.Tasks' invariant
finding: 'recordFailure''s godoc states it must not write state.Tasks, since the schedule is evaluated against the last successful run. Nothing enforces it. Verified by mutation: injecting s.state.Tasks[task.Name] = start into recordFailure leaves the whole package green. The ladder gate masks the corruption while NextRetry is set, so it would surface only later via any consumer reading Tasks as ''last successful run''.'
severity: critical
resolution: 'Added TestDoExecuteTask_failureDoesNotCountAsRun, which runs a genuinely failing Lua script through the real doExecuteTask and asserts state.Tasks has no entry, Failures==1, and NextRetry is based on start (not completion). Verified by re-running the mutation: injecting the state.Tasks write into recordFailure now FAILS this test.'
status: addressed
---
