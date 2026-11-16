---
id: RR-CPNYIS8
type: review-response
title: Job redelivery duplicates writes
finding: The queue delivers at least once; a run whose completion was not recorded ran again and created its entities twice.
severity: critical
resolution: Each enqueue carries a run token; after a successful apply the handler records it in state.KV under a hash of the idempotency key and a redelivery with the same token returns without running.
status: addressed
---
