---
id: RR-7C9B4Q
type: review-response
title: Every handler silently capped at neoq's default 30s timeout
finding: handler.New was called with only Concurrency, so neoq applied its DefaultHandlerTimeout of 30s. The slow external I/O this package exists to carry — LLM calls, slow SMTP — would be killed at 30s and counted as a failed attempt, driving jobs toward exhaustion (and, before the fix, toward worker death). Nothing in the code or docs said so, and Job.Deadline reads as if it were the time bound.
severity: critical
resolution: handler.JobTimeout is now set explicitly to handlerTimeout (15m), documented as a backstop against a wedged handler rather than a latency target — real bounding is the job's deadline and retry budget. The constant carries the reasoning so the inherited default cannot silently return.
status: addressed
---
