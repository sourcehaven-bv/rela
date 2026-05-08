---
id: RR-6F1AF
type: review-response
title: Required-property deletion is unguarded
finding: 'internal/mcp/tools_entity.go:218-224. validatePropertyNames only checks names exist in the metamodel — it doesn''t enforce Required. update_entity with {"title": null} on an entity whose title is required will succeed at the MCP layer and report ''Updated REQ-001'' with a now-invalid entity. analyze_validations catches it later, but the user gets a misleading success first. Either reject required-prop deletes at merge time with an actionable error, OR document explicitly in the tool description. (The plan''s stated position was ''do not gate, let analyze catch it'' — reaffirm or change.)'
severity: significant
resolution: 'Reversed the plan''s earlier ''don''t gate'' decision — the cranky reviewer''s UX point won. Extended validatePropertyNames in tools_helpers.go to reject nil values targeting Required:true properties with an actionable error: ''cannot delete required properties for <type>: <names> (set a new value instead)''. The user gets a clear error at the MCP boundary instead of a misleading ''Updated'' that analyze_validations later catches. Added TestHandleUpdateEntity_DeleteRequiredPropertyRejected confirming title (required) cannot be deleted on requirement entities.'
status: addressed
---
