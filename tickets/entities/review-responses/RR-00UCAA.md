---
id: RR-00UCAA
type: review-response
title: numericValue accepts hex and exponent strings
finding: Number() accepts 0x1F, 1e3 and padded strings, which the doc comment did not say.
severity: nit
resolution: numericValue accepts only plain decimal strings; table test cases for hex, exponent and padding.
status: addressed
---
