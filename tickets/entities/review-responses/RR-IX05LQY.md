---
id: RR-IX05LQY
type: review-response
title: No write-time validation of the condition property
finding: '[security] An edge may be saved with a malformed valid_until.'
severity: minor
reason: A malformed or missing value grants nothing (shape guard in Go and SQL); write-time 422 is a usability follow-up and not needed for safety.
status: deferred
---
