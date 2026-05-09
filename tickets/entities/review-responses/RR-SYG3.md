---
id: RR-SYG3
type: review-response
title: analyzeProperties false-positives — reports encrypted entities as missing required fields
finding: 'internal/dataentry/analyze.go:384-403 analyzeProperties walks store.ListEntities directly and calls Meta.ValidateEntity(e.ID, e.Type, e.Properties). For an encrypted entity, e.Properties is empty, so every required-field check fires false-positive ''missing title/status/...'' errors. This drives the data-entry Analyze view AND the rela-issues MCP `analyze_properties` tool used in the ticket-done workflow. The validator skip in loadCandidates does not protect this path. Same gap likely affects analyzeOrphans and analyzeCardinality for property-derived rules. Fix: skip entities with non-empty Inaccessible at the top of analyzeProperties (and audit other analyzers in the same file). Better: factor ''should this entity be evaluated by property-driven rules?'' into a single helper used everywhere.'
severity: critical
status: open
---
