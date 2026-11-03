---
id: RR-KE0KI2
type: review-response
title: toneHues duplicated SemanticTones
finding: A tone added to SemanticTones would validate but paint nothing.
severity: minor
resolution: toneHues moved to colors.go with ToneColor(name); SemanticTones is derived from it, so the two cannot drift.
status: addressed
---
