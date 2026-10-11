---
id: RR-KEVEXR
type: review-response
title: Validator rebuilt per loop iteration
finding: NewValidator called inside loops in dataentry/analyze.go, mcp/tools_analysis.go, schema/validate_properties.go, dataentry/relations_modern.go and twice in updateCore.
severity: nit
resolution: Hoisted one metaValidator above each loop and once in updateCore.
status: addressed
---
