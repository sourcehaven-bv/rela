---
id: RR-7PU1A92
type: review-response
title: 'The #2106 check only refuses aliases'
finding: An undeclared type in a client_baselines redact key still loads, and scope_grants keys are unchecked.
severity: minor
reason: 'A different failure class (typo, not alias) that predates this ticket; aclaudit reports it. Out of scope for the #2106 regression check.'
status: deferred
---
