---
id: RR-DZV8FFZ
type: review-response
title: enabled with no value hands the decision to flag and env
finding: 'remote_mcp.enabled: with a commented-out value decoded to nil.'
severity: minor
resolution: enabled refuses a null value like allow; test added; docs list it.
status: addressed
---
