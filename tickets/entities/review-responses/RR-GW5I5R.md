---
id: RR-GW5I5R
type: review-response
title: '''legacy alias'' comments overstate history (description was never rendered)'
finding: Comments in config.go, EntityList.vue, config.ts call `description` the 'legacy name' that 'old configs keep rendering', but description was never rendered before this change. Reframe to 'we adopt the previously-unused description field as a fallback' to avoid implying a backward-compat contract that never existed.
severity: minor
resolution: Reframed comments in config.go, config.ts (both the field doc and helper doc), and EntityList.vue to say `description` is a previously-unused field adopted as a fallback, not a rendered legacy behavior being preserved.
status: addressed
---
