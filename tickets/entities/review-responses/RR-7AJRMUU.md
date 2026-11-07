---
id: RR-7AJRMUU
type: review-response
title: Invalid PROP_SEED not rejected
finding: PROP_SEED= becomes 0 and PROP_SEED=abc becomes NaN.
severity: minor
resolution: A PROP_SEED that is not an integer throws.
status: addressed
---
