---
id: RR-S26LON
type: review-response
title: Comment lines not reflowed
finding: 'internal/apiwire/v1/responses.go:185 and internal/dataentry/realtime_wiring.go:49 run past the surrounding wrap width. internal/cli/validate.go:578 leaves a dangling "It" line. docs-project/entities/guides/GUIDE-realtime.md:1753 is an overlong prose line. Fix: reflow each paragraph.'
severity: nit
resolution: Reflowed the comments in responses.go, realtime_wiring.go and validate.go, and the guide line.
status: addressed
---
