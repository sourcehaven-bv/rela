---
id: RR-RUI15
type: review-response
title: ID property cannot be deleted but plan does not say so
finding: 'validatePropertyNames at tools_helpers.go:107-110 only checks the map against entityDef.Properties. The ''id'' field of the entity is not a metamodel property — it''s a top-level attribute. But what about title, status, and other metamodel-defined props that the entity needs to be valid? The plan correctly says ''we do not gate on required-prop deletion; let analyze_validations catch it''. But there''s a sharper case: what if a client sends {"id": null}? validatePropertyNames would reject ''id'' as unknown (not in entityDef.Properties), so this is implicitly safe — but only IF id is genuinely not in the metamodel properties map for any entity type. Verify this assumption (search for an entity type that defines ''id'' as a metamodel property; if any exist, add an explicit guard). Also document in the tool description: ''id cannot be deleted''.'
severity: significant
resolution: 'Verified: `id` is not declared as a metamodel property in any entity type (grep of tickets/metamodel.yaml shows no `id:` property entries). validatePropertyNames will reject `{"id": null}` as an unknown property. No code change needed; plan updated to call out the assumption.'
status: addressed
---
