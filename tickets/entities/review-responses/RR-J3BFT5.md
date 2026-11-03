---
id: RR-J3BFT5
type: review-response
title: lifecycleText uses a positional [2]string
finding: internal/openapi/channels.go:175 indexes text[0]/text[1] for summary and description. Use a two-field struct so the two strings cannot be swapped silently.
severity: nit
resolution: Replaced it with a lifecycleDoc struct with summary and description fields.
status: addressed
---
