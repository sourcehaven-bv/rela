---
id: RR-Y7CJLD
type: review-response
title: Worker pool dies permanently when a job exhausts its retry budget
finding: 'neoq''s worker goroutine RETURNS when handleJob yields ErrJobExceededMaxRetries (memory_backend.go:245-250). Handing neoq the real per-policy budget therefore costs one worker per exhausted job until the pool is empty and the queue is permanently silent — with no error surfaced to rela and nothing in the logs. Independently reproduced: 4 exhausted RetryNever jobs, then 0 of 8 healthy jobs ran. The original MaxRetries=1 mapping did not fix this; it only moved the death from attempt 1 to attempt 2.'
severity: critical
resolution: Inverted the ownership. neoq is now handed backendRetryBudget (1<<30) — a budget no job can reach — and the real attempt accounting moved into neoqQueue.dispatch, which tracks attempts in the payload (__rela_attempt) and returns nil once the budget is spent, so neoq never evaluates its own terminal condition. Pinned by jobstest PoolSurvivesExhaustedJobs, which was verified to FAIL with the exact diagnostic when backendRetryBudget is reverted to 1.
status: addressed
---
