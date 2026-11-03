---
id: RR-X2JWTA
type: review-response
title: Test gaps for create, desc truncation, hidden declared range, bad bound
finding: No POST test, no desc+truncation test, no all-hidden declared range test, no non-numeric bound load test, no integer-with-values test.
severity: minor
resolution: 'Added all except integer-with-values: values: has no meaning on an integer (GetValidEnumValues only reads enum and custom types), so the case does not exist.'
status: addressed
---
