---
id: RR-N9M1S4
type: review-response
title: Missing checker binary fails once per entity
finding: collectProse never probes; a missing vale yields one identical error per entity.
severity: significant
resolution: collectProse probes every analyze checker before the loop and returns one error naming the checker; TestAnalyzeProse_MissingBinaryFailsOnce.
status: addressed
---
