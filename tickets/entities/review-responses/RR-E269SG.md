---
id: RR-E269SG
type: review-response
title: No per-channel bound on system posts, and one rule can starve the rest
finding: System posts bypass the rate buckets, so an entity that flips state each tick (an integration sync) posts about 2 messages a second into its channel forever. `MaxSystemPostsPerRound` is global, so one noisy rule starves all others. Add a per-(rule, entity) cooldown or a system bucket per channel, and round-robin the cap across rules.
severity: minor
resolution: 'Per D11: a per (rule, entity) cooldown of one post per minute with coalescing, and the 64 cap is shared round-robin across rules.'
status: addressed
---
