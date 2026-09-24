---
id: RR-8TNTL0
type: review-response
title: Lower does not deduplicate traversals
finding: A repeated related() was gated and joined twice, unlike answerTraversals.
severity: nit
resolution: Lower skips repeats by spec.Key(); unit case asserts one gate call and one entry.
status: addressed
---
