---
id: RR-BJW16
type: review-response
title: Plan passes req.ID unconditionally to CreateOptions, conflating manual-ID with short/sequential
finding: 'For a short/sequential type the form should NOT send `id` at all. If a malicious or buggy client sends `{id: ''TKT-HACKED''}` for a type with `id_type: short`, the backend today (api_v1.go:392 `CreateOptions{ID: req.ID}`) happily creates an entity with that ID, bypassing the entire short-ID generation. This is pre-existing behaviour but the plan does not address it and multi-prefix makes it worse (user could send `{id: ''WRONG-001''}` ignoring the prefix rules). Recommend: server validates that `req.id` is only honored when `entityDef.IsManualID()`; otherwise 422 ''ID not accepted for non-manual ID types''. Document the decision in the plan.'
severity: significant
resolution: Plan now explicitly rejects `req.id` for non-manual types (422) and `req.prefix` for manual types (422). Applies to both handleV1CreateEntity and legacy handleAPICreateEntity.
status: addressed
---
