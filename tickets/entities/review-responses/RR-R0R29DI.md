---
id: RR-R0R29DI
type: review-response
title: 'Code: range bounds not checked for UTF-8 or NUL'
finding: A peer bound with invalid UTF-8 or NUL fails on pg with a driver error but is answered by sqlite and elementnaive.
severity: minor
resolution: validateElementRange refuses such bounds with ErrInvalidQuery; conformance cases bound_not_utf8 and bound_nul.
status: addressed
---
