---
id: RR-JT4ZA9
type: review-response
title: compose && generation === undefined opens a plain form instead of failing closed
finding: useSessionDecisions.ts:433 skips recording when generation is undefined, then line 449 returns true. A compose view would open a plain type form whose Send carries no preset and no generation. onDecide passes ?? 0, so this is unreachable today, but it is a silent fallback the next caller can hit. Treat a compose view with no generation as stale.
severity: nit
resolution: "addressed\tA compose view with no generation now goes through the stale path and records nothing, instead of opening a plain form."
status: addressed
---
