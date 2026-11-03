---
id: RR-EHBB1UG
type: review-response
title: File-store segment cost not measured
finding: File GetMessages parses the whole segment (up to 1000 messages) per channel; the benchmark seeds one message per channel so it cannot see this.
severity: significant
resolution: AC 7 seeds each waiting channel with about 700 messages before a 4 KB cause and records file and postgres numbers; fallback is a single-line seek inside the segment if the 4 s grid is missed.
status: addressed
---
