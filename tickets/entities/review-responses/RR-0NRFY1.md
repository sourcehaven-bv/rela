---
id: RR-0NRFY1
type: review-response
title: Retry budget is 3 ticks (about 1.5 s), so a short outage permanently loses notifications
finding: 'internal/realtime/trigger.go:906-917 (postOne). A failing Append is requeued for the next tick (500 ms). After TriggerRetries=3 failures the effect is applied, so the change counts as seen and is never posted. A database failover, a lock timeout or a pg restart that lasts more than about 1.5 s therefore silently drops every trigger message due in that window, and reconcile cannot recover them because the seen-set already records them. D6 asks for bounded retries, but the bound should cover realistic outages and should not count deterministic and transient failures alike. Fix: space retries by time (for example exponential backoff from TriggerTick up to a few minutes, through postpone() rather than requeue()). Give up only on deterministic errors (validateProps, whenHolds, ErrTypeShapeConflict) or after a time bound. Do not count ctx.Err() as a failure.'
severity: significant
resolution: Transient post failures are retried with a delay that doubles from the tick up to one minute, for a five minute window (TriggerRetryWindow). Errors that can never succeed are given up at once, and context errors do not count.
status: addressed
---
