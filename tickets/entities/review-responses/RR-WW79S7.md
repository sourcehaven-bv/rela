---
id: RR-WW79S7
type: review-response
title: invalidate can roll back a rotated refresh token
finding: Unlocked get-clear-put can write back a revoked refresh token.
severity: critical
resolution: 'Plan R2: compare-and-clear under the refresh lock; CLI set/delete take the lock.'
status: addressed
---
