---
id: RR-4I8XBU
type: review-response
title: ETag rule can serve a stale lease via 304
finding: 'Security design review: If-None-Match still answers 304 with an old _lease.'
severity: minor
resolution: 'Plan: keep the ETag; skip 304 while a lease is present.'
status: addressed
---
