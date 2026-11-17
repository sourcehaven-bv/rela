---
id: RR-26H90JQ
type: review-response
title: Test misses all-zeros and signed long boundary inputs
finding: No case for 200000 zeros only, or a sign, many zeros and a 19-digit value just out of range.
severity: minor
resolution: Added both cases to TestIntegerExprAgreesWithGo.
status: addressed
---
