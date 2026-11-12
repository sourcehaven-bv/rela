---
id: RR-K3DMOQ1
type: review-response
title: Outcome compare-and-set drops results on any concurrent run write
finding: HandleAttempt CAS on the rev read before a run of up to 10m loses to any other write (another branch, backoff), dropping a real outcome.
severity: critical
resolution: 'Plan: the start and outcome writes loop on conflict, re-reading and checking token, generation, seq and phase; only a mismatch or a gone run stops them. Test with a concurrent branch write.'
status: addressed
---
