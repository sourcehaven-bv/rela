---
id: RR-JXSNFY
type: review-response
title: dispatch dropped the enqueued Retry policy, misreporting every job as RetryNever to handlers
finding: The Job handed to a handler was built as Job{Kind, Payload} with Retry left at its zero value — RetryNever — even though retryOf had just read the real policy two lines earlier. A handler inspecting job.Retry (a reasonable thing to do with an exported field on the struct it receives) saw 'never' for every job. Silent misinformation is worse than absence.
severity: significant
resolution: The reconstructed Job now carries both the real Retry and the effective Deadline. Pinned by jobstest HandlerSeesItsRetryPolicy, which enqueues RetryPersistent and requires the handler observes RetryPersistent.
status: addressed
---
