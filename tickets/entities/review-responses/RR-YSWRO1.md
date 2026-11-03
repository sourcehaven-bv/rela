---
id: RR-YSWRO1
type: review-response
title: CLI prints a list property type as "array"
finding: internal/cli/channel_render.go:216-228 prints `p.Type`, which becomes `array`. Print `list of string` (and max items) so the agent-facing text matches the config vocabulary.
severity: nit
resolution: The CLI prints a list property as "list of string, at most N" (step 6, AC 3).
status: addressed
---
