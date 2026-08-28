---
id: RR-JPYXQ9
type: review-response
title: id prefixes in ShapeProjection but no step can migrate ids
finding: 'The plan includes id prefixes in ShapeProjection, so an id_prefix change flips the shape hash and classifies as needs-migration — but the v1 step vocabulary has no operation that can rewrite entity IDs (a reprefix requires per-entity store.RenameEntity plus relation re-keying). The result is an unresolvable needs-migration state: the gate warns forever and no generatable migration can clear it. Either exclude id_prefix from the v1 projection (id-prefix changes remain the existing config-migration concern, as today) or add a reprefix step — the plan must pick one.'
severity: significant
resolution: 'Amendment A5: id prefixes are EXCLUDED from the v1 ShapeProjection, so a prefix change never enters the migration system (it remains the existing config-migration concern, as today). A future reprefix_ids step may re-add them; noted in out-of-scope. AC1 updated to assert prefix changes don''t move the hash.'
status: addressed
---
