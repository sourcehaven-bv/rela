---
id: RR-2O56DAS
type: review-response
title: Nav status rule counts are not one snapshot
finding: 'cranky: each rule is its own CountMatched, so a write between counts can change which rule wins; the godoc claimed the same counts as Go.'
severity: minor
resolution: 'Addressed: the navStatusInStore godoc states it and why it is accepted (an advisory marker, corrected on the next request).'
status: addressed
---
