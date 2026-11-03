---
id: RR-NMR7U8
type: review-response
title: Version polling parses the whole document
finding: 'Architecture design review: up to ~600 KB per second on kv.'
severity: minor
resolution: 'Plan: tiny realtime-leases.version key written under the lock; pg uses a cheap max(seq).'
status: addressed
---
