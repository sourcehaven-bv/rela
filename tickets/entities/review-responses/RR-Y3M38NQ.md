---
id: RR-Y3M38NQ
type: review-response
title: Prune holds the chain lock for the whole DELETE
finding: A large prune blocks appends for its duration.
severity: minor
reason: Retention runs rarely and the sink buffers appends meanwhile. Chunked pruning can follow if it shows up in practice.
status: deferred
---
