---
id: RR-MBRPODF
type: review-response
title: Decimal sort parses values at every comparison
finding: CompareDecimals builds two big.Rat per comparison on the Go sort path.
severity: minor
reason: Deferred to a follow-up decimal ticket; correct but slow on large lists; parse once per sort as a performance follow-up.
status: deferred
---
