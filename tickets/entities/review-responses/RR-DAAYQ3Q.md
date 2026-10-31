---
id: RR-DAAYQ3Q
type: review-response
title: Nil rows reach the prime
finding: 'Design review: primeVerdicts reads e.ID without a nil check; a nil row in a page would panic the prime.'
severity: minor
resolution: redactingSeq skips nil rows and primePage skips nil ids. TestRedactingSeq_SkipsNilRows.
status: addressed
---
