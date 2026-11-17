---
id: RR-O9NYUF7
type: review-response
title: Unbound instant must deny
finding: A zero time.Time makes today 0001-01-01, so valid_until >= today matches every expired edge.
severity: significant
resolution: 'Plan: a zero instant makes every conditional edge non-matching in Go and store; tested per binder.'
status: addressed
---
