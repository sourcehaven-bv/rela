---
id: RR-6INQHE
type: review-response
title: A late report from an abandoned run was delivered to a later run of the same task
finding: The in-flight registry is keyed by task name alone. When a submitter returned via ctx cancellation while its worker was still executing, its deferred releaseInFlight freed the claim; a later run then installed a fresh channel under the same name. When run N's worker finally finished, reportInFlight looked up by name, found run N+1's channel, and delivered run N's outcome to it — so the scheduler stamped the wrong result (recordSuccess/recordFailure) for the wrong run. The existing doc reasoned only about a MISSING entry and never considered a DIFFERENT entry under the same name.
severity: critical
resolution: 'Each run now carries a process-local token (a monotonic counter, since the token is only ever compared against this process''s own map and never persisted). reportInFlight delivers only on a token match and releaseInFlight evicts only its own claim, so a stale submitter can neither hijack a later run''s result nor drop its claim. Pinned by two regression tests verified to fail against the reverted code with exactly the predicted symptoms. Note: a reviewer claim that this also caused two concurrent runs of the same script was tested and disproved — the idempotency key does suppress a duplicate while the first run is executing.'
status: addressed
---
