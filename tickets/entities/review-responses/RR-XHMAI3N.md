---
id: RR-XHMAI3N
type: review-response
title: max_tokens is per run, not per row
finding: An operator who sets max_tokens for one entity and adds batch gets cut-off replies that fail the whole run.
severity: minor
resolution: The guide says max_tokens caps the whole answer and must be set for max entities.
status: addressed
---
