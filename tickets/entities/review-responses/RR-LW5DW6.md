---
id: RR-LW5DW6
type: review-response
title: 'Head-of-line blocking: one undeliverable message stalls the queue ~30s at production backoff'
finding: 'run is a single goroutine and deliver sleeps the whole loop through the backoff, so a second perfectly deliverable message waits behind a failing one. At shipped defaults (5 attempts, 2s/4s/8s/16s) that is ~30s of stall per poison message, and a burst of bad addresses serializes: N x 30s. A single typo''d recipient in an entity property — exactly the input this receives — blocks every other notification behind it, and with Capacity 128 a digest run can fill the buffer and start returning ErrOutboxFull for good messages because of one bad one. TestOutbox_GivesUpAfterMaxAttempts uses a 5ms backoff, which hides the magnitude entirely; its comment claims a property the code only satisfies eventually.'
severity: significant
reason: 'Real and correctly diagnosed, but fixing it properly means distinguishing permanent SMTP failures (5xx, invalid recipient) from transient ones, or moving to requeue-with-deadline instead of sleeping the consumer. Both are queue-semantics work that belongs with the durable backend (IDEA-WIJ2H1), where concurrent consumers and per-message state make it natural — not bolted onto an in-memory buffer this ticket documents as best-effort. Deferring rather than half-fixing: a partial retry-classification would look like a solution while still serializing on the transient cases. Documented in the package doc and the operator guide so the stall is a known cost rather than a surprise, and IDEA-WIJ2H1 carries it forward.'
status: deferred
---
