---
id: RR-BG15A5
type: review-response
title: Conformance gaps in leasetest
finding: 'Code review: no concurrent per-identity cap test, no mixed concurrent operations, whole-second instants hide kv/pg precision differences, no Rekey edge cases, no version assertions for refused claims.'
severity: significant
resolution: 'Fixed in 53753571 (Lease storage: review fixes); see the commit for the change and its conformance test.'
status: addressed
---
