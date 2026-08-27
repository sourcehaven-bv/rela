---
id: RR-MCAOI3
type: review-response
title: 'Silent job loss: neoq deduplicates by payload hash, Enqueue returns nil having queued nothing'
finding: 'neoq fingerprints a job as md5(queue + json(payload)) and drops any job matching an unprocessed one, returning DuplicateJobID with a NIL error. Every rela job shares one queue, so two logically distinct jobs with equal payloads collapsed into one — ''notify alice'' submitted by two triggers loses one delivery silently. The tiers also disagreed: memory drops silently, postgres has a unique partial index and returns an error, so identical calls behaved differently per deployment.'
severity: critical
resolution: 'Enqueue now sets Fingerprint explicitly to a fresh UUID per job (FingerprintJob no-ops when already set), disabling payload-equality dedup on both tiers. Pinned by jobstest IdenticalPayloadsAreDistinctJobs: 5 enqueues of an identical payload must produce 5 executions. If dedup is ever wanted it becomes an explicit opt-in field on Job, not an accident of payload equality.'
status: addressed
---
