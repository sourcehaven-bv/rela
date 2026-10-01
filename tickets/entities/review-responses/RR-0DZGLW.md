---
id: RR-0DZGLW
type: review-response
title: Bug description understates impact
finding: A process with a pool-tuned DSN never resolves its schema, so it also never publishes NOTIFYs; peers miss its writes until their catch-up.
severity: minor
resolution: Bug body updated.
status: addressed
---
