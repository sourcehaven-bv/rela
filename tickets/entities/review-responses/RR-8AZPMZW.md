---
id: RR-8AZPMZW
type: review-response
title: Constant Retry-After on held failures
finding: Responses during a failure hold always sent the same Retry-After value regardless of how much of the hold was left.
severity: nit
resolution: 'Fixed in 13cd61e18: Retry-After is the remaining hold time.'
status: addressed
---
