---
id: RR-OANV126
type: review-response
title: tenant show could print the derived schema before the control table exists
finding: showTenant computes the schema before connecting but discards it when the lookup fails, so the guide works around the command.
severity: minor
reason: 'Issue #2172 allows either fix; this PR fixes the guide. Changing tenant show output is a separate behaviour change to the operator CLI.'
status: deferred
---
