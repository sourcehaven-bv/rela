---
id: RR-26WKZX
type: review-response
title: Alias check covers only half of design A2
finding: app.default_world is checked only when schema default_world is set, not against the effective default world (cranky review).
severity: minor
resolution: validateDefaultWorldAlias checks app.default_world against metamodel.EffectiveDefaultWorld (schema key, else first declared world). A declared but non-first world is refused; pinned in validate_test.go.
reason: app.default_world stays authoritative until PR 5a applies the schema key, so the check belongs with that switch. Recorded in the PR 5a scope.
status: addressed
---
