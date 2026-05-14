---
id: RR-C7TE6
type: review-response
title: Required-boolean-false re-verify and add regression test
finding: 'Test plan claims required-boolean-false works. Verified: line 43 check is ''!exists || val == nil || val == empty || isEmptyList''. Go bool false matches none. Form-submission ''false'' string matches none either (treated as set). Correct today. BUT: required boolean stored as Go false with omitempty YAML serialization disappears from disk; on reload val == nil fires required. Existing latent bug — not introduced here. Post-softening: every save of entity with required-false-boolean adds required_property_unset warning on every PATCH. Recommendation: single regression test — entity with required boolean=false, save, reload, PATCH unrelated property, assert no required warning for boolean. If fails, file the storage-omitempty bug separately. From design-review F12.'
severity: minor
resolution: 'AC30 added: required boolean=false regression. Save, reload, PATCH unrelated property, assert no required_property_unset warning for the boolean. If test fails, file the storage-omitempty bug separately as out-of-scope.'
status: addressed
---
