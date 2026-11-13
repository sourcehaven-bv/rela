---
id: RR-GD3NSCS
type: review-response
title: HTTP body limit too small for a batch
finding: channelBodyLimit is MaxMessageBytes plus 1 KiB, about 17 KiB, while a batch of 10 can be about 160 KiB.
severity: significant
resolution: 'Plan: a separate batch endpoint with its own limit (MaxBatch*MaxMessageBytes plus overhead) and a total-bytes cap per batch, tested.'
status: addressed
---
