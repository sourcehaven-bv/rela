---
id: RR-CQYAQ14
type: review-response
title: fake.random could panic on a large interval
finding: An interval of MaxInt width panicked inside the random source.
severity: minor
resolution: luaRandom raises interval is too large.
status: addressed
---
