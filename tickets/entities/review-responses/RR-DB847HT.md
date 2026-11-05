---
id: RR-DB847HT
type: review-response
title: The resolve 422 did not name the failing property
finding: With the value gone from the message the 422 detail read only the rule. The caller could not tell which property failed.
severity: significant
resolution: 'conflict.ValidateResolved prefixes the property name. TestV1ConflictResolve_ValidationErrorHidesValue asserts points: Out of range.'
status: addressed
---
