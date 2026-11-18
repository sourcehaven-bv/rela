---
id: RR-Z2ME48X
type: review-response
title: Sink retries are too short and unbounded in memory
finding: Three quick retries lost records on a short database outage; the queue had no cap and Close could hang on a stuck database.
severity: significant
resolution: Batches of 500 per transaction, a 10s per-attempt timeout, about 20s of backoff, a 100000-record queue cap with counted drops, and Close cancels after 10s and returns the unwritten count. Pinned by the sink tests.
status: addressed
---
