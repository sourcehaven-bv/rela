---
id: RR-CLASPN
type: review-response
title: Infinity passes as an integer
finding: YAML .inf saturates to MaxInt64 and is accepted without a max.
severity: nit
resolution: validateIntegerValue rejects +-Inf as an invalid integer. Test added.
status: addressed
---
