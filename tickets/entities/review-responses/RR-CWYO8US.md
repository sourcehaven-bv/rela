---
id: RR-CWYO8US
type: review-response
title: Combined PATCH stored the update before its counted edges (fail-open)
finding: The PATCH judged the entity update on pending edges, committed the update, and only then wrote the edges. If a counted edge write failed (ACL, bound, missing target, far-end rule) the entity was left breaking an enforced rule. Found by the automated security review and the PR guidance check (G-005).
severity: significant
resolution: 'New entitymanager.PatchWithRelations: the replace is authorized and validated before the patch, and inside one store Tx every check of both writes runs before the entity and then the edges are written. The data-entry PATCH uses it for wrappers a rule reads. WithPendingRelations is removed. Pinned by TestPatchWithRelations (ACL denial, far-end refusal, near-end refusal, accepted cases, mem and fs) and the far-end case in TestRelationRule_PatchJudgesTheStateItLeaves.'
status: addressed
---
