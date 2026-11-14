---
id: RR-4VP4TED
type: review-response
title: Page memory and early exits for full entities
finding: A page sized to the batch holds every requested entity body at once, and an early-exiting caller pays for rows it never reads.
severity: minor
resolution: 'ListEntities uses bodyPage: one-shot past the threshold but on the fixed page. Only headers and relations get a batch-sized page.'
status: addressed
---
