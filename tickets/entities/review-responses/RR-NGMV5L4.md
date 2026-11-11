---
id: RR-NGMV5L4
type: review-response
title: Lifting a stuck restriction is described wrongly
finding: A header can stay unfinished with no stored run after a failed cancel; the plan said cancel lifts it.
severity: minor
resolution: 'Addressed: the ceiling reads the run store, so a header without a stored run restricts nothing.'
status: addressed
---
