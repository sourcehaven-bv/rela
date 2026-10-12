---
id: RR-KNHSAA
type: review-response
title: Lease request JSON decoding is not strict
finding: json.Unmarshal accepts unknown fields so a misspelled ttl silently claims for the default; use DisallowUnknownFields and reject trailing data
severity: significant
resolution: 'Fixed in the review-fix commit (Lease HTTP: review fixes) with tests where behaviour changed.'
status: addressed
---
