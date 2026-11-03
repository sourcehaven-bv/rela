---
id: RR-3V0R6O
type: review-response
title: CLI set line omits the face
finding: 'internal/cli/channel_render.go outputsText prints "set <anchor>" with the channel anchor. For a family-anchored type with set.face it names the bare id, not the face written. Fix: render the set target with its face when set.face is configured.'
severity: nit
resolution: Results carry face on wire, OpenAPI and TS; the CLI prints id@face; render test extended.
status: addressed
---
