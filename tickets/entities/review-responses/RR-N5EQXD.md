---
id: RR-N5EQXD
type: review-response
title: 30 s reaper
finding: 'Architecture design review: expiry is lazy; a reaper only wakes and cleans.'
severity: significant
resolution: 'Plan: Watch arms a timer at the earliest expiry; Claim prunes expired entries; Reap removed from the interface.'
status: addressed
---
