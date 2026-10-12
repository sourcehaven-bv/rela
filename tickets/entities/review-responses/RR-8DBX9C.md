---
id: RR-8DBX9C
type: review-response
title: Presence 404 parity test misses malformed and world cases
finding: parity holds by construction but the test would not catch a handler-side regression
severity: minor
resolution: 'Fixed in the review-fix commit (Presence: review fixes) with tests; presence tests pass 50 of 50 runs under race.'
status: addressed
---
