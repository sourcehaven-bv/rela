---
id: RR-JO296I
type: review-response
title: Post-commit retention enqueue collapses into a running job so delete is not prompt
finding: jobs.Job.IdempotencyKey matches until the job finishes, so an EntityDeleted enqueue during a running retention pass (which already listed Tombstones) is dropped and the batch waits up to 1h. The alias hook also cannot know the deleted anchor's channel types after the guard tombstoned them. Enqueue a per-handle purge job, or have the retention pass loop until no batch is due, and enqueue whenever any delete-policy type is configured.
severity: minor
resolution: Background work runs in the single leader worker (T5 D1). The post-commit hook wakes it, and a retention pass loops until no batch is due.
status: addressed
---
