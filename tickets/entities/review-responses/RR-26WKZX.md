---
id: RR-26WKZX
type: review-response
title: Alias check covers only half of design A2
finding: app.default_world is checked only when schema default_world is set, not against the effective default world (cranky review).
severity: minor
reason: app.default_world stays authoritative until PR 5a applies the schema key, so the check belongs with that switch. Recorded in the PR 5a scope.
status: deferred
---
