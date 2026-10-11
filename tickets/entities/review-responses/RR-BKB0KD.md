---
id: RR-BKB0KD
type: review-response
title: Pull writes trigger push storm
finding: Every pull write fires a background push, exhausting the rate limit.
severity: significant
resolution: 'Plan R7: push compares ours with base locally first and makes no HTTP call when nothing differs; stub counts calls.'
status: addressed
---
