---
id: RR-CPNYIS8
type: review-response
title: Job redelivery duplicates writes
finding: The queue delivers at least once; a run whose completion was not recorded ran again and created its entities twice.
severity: critical
resolution: Each enqueue carries a random run token. EnqueueLLMAction writes an empty marker at llm-action-jobs/runs/<token> in state.KV before it queues the job, and deletes it again if the enqueue fails. The handler runs only while the marker exists. It deletes the marker after a success, after a failure that is not retried, after a retryable failure on the final attempt, and when the user no longer resolves. A retryable failure with attempts left keeps the marker, so the next attempt runs. A redelivery that finds no marker returns without running.
status: addressed
---
