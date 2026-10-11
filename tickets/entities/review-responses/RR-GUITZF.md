---
id: RR-GUITZF
type: review-response
title: expires_in accepts NaN, Inf and huge values
finding: ParseFloat accepts non-finite values; Duration conversion is undefined.
severity: minor
resolution: parseExpiresIn refuses non-finite values and caps at one year; tests for NaN, Inf, -Inf, 1e300 and negative.
status: addressed
---
