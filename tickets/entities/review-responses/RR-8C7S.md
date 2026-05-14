---
id: RR-8C7S
type: review-response
title: 'Cranky #6: wsScriptRunner missing compile-time interface assertion'
finding: wsScriptRunner had no `var _ autocascade.ScriptRunner = (*wsScriptRunner)(nil)` — interface changes would surface at the wiring site instead of the type.
severity: minor
resolution: Added the assertion at internal/workspace/wsscriptrunner.go:26.
status: addressed
---
