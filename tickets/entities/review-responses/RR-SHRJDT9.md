---
id: RR-SHRJDT9
type: review-response
title: A nil error is wrapped
finding: An error path wrapped a nil error in the MCP files argument handling.
severity: nit
resolution: The over-limit branch is split out and returns its own error.
status: addressed
---
