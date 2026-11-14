---
id: RR-UZRJHVZ
type: review-response
title: t.flows.work lost queued attempts when one failed
finding: take() emptied the queue before running; an error raised mid-loop dropped the rest, and a later t.flows.work after pcall skipped them. The ignored dedupe key was undocumented.
severity: minor
resolution: The attempts not run are requeued before raising. EnqueueAttempt documents that HandleAttempt is idempotent, so the dedupe key is not needed.
status: addressed
---
