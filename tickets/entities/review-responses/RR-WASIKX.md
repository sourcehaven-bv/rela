---
id: RR-WASIKX
type: review-response
title: Non-deterministic responses would leak hidden write timing
finding: A re-read runs after every change; any byte that varies between runs would send a frame after hidden writes, revealing write timing that polling does not.
severity: significant
resolution: 'AC4 now requires stable hashes: per-shape test of two rounds without a write and a hidden write, including an entity GET of a hidden id. Only status and body are hashed.'
status: addressed
---
