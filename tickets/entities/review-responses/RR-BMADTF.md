---
id: RR-BMADTF
type: review-response
title: 409 reconnect loop has no backoff
finding: 'Code review: a persistent principal mismatch would loop connect, ready, PUT, 409, reconnect without delay, since ready resets the stream backoff.'
severity: nit
resolution: The manager backs off 409 reconnects (1 s doubling to 30 s), reset on a successful PUT. Pinned in useLiveSubscription.test.ts.
status: addressed
---
