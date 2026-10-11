---
id: RR-LPGND4
type: review-response
title: Unit tests cannot detect wrong centring
finding: jsdom clientWidth 0 made centring untestable.
severity: minor
resolution: Tests stub .chart clientWidth to 880 and assert the centred offset; stale-response, one-day bar and compression tests added.
status: addressed
---
