---
finding: Plan mentions `RandomInt(min, max int)` but doesn't specify behavior when min > max. Should panic or swap values? Add to edge cases.
id: RR-s49e
resolution: RandomInt will swap min/max if min > max to avoid panic. This is more ergonomic for test code.
severity: nit
status: addressed
title: 'RandomInt edge case: min > max'
type: review-response
---
