---
id: RR-Z2I3WH
type: review-response
title: Conformance suite asserted every property except that the queue still works
finding: All thirteen original jobstest cases tested the happy path of a property and then stopped. None asserted the queue was still processing afterwards, so both critical worker-death bugs were invisible to a green suite — the worst possible property for a conformance harness, because green was actively misleading.
severity: significant
resolution: 'Added four cases, of which PoolSurvivesExhaustedJobs and PoolSurvivesExpiredDeadlines are the load-bearing ones: run a batch of failing/expiring jobs, THEN enqueue healthy ones and require they all run. Verified these actually bite — reverting backendRetryBudget to 1 makes PoolSurvivesExhaustedJobs fail with ''worker pool died''. Also added IdenticalPayloadsAreDistinctJobs and HandlerSeesItsRetryPolicy.'
status: addressed
---
