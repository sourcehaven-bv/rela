---
id: RR-5V6CN
type: review-response
title: Per-PATCH warning scoping is the wrong direction — keep full post-write state
finding: 'Plan author leaned full-state but proposed hybrid rule (warn on properties touched OR Required regardless). Hybrid breaks user mental model: PATCH status on entity with stale unknown_xyz suppresses unknown-key warning but surfaces required-title warning. Why? ''Required is special.'' Not internalizable. Not idempotent: PATCH {status} and PATCH {title} produce disjoint warning sets — confusing. Implementation has to thread req.Properties keys to validator — leaks request boundary into validator. Auto-save fatigue is a UI concern; solve in TKT-E6094. API contract is ''what''s wrong with this entity now'' — suppressing pre-existing warnings to soften UX is data-hiding. Recommendation: emit warnings for every soft validation error on post-write entity, full stop. Risk #5 ''spam'' belongs to autosave UX. From design-review F5.'
severity: significant
resolution: 'Warning scope locked to ''post-write entity full state'' (no per-PATCH scoping). Risk #5 hybrid-rule removed entirely. Out-of-scope notes that auto-save warning fatigue is TKT-E6094''s problem to solve in UI (debounce, dismiss, show-only-new). API contract is ''what''s wrong with this entity now'' — suppressing pre-existing warnings is data-hiding.'
status: addressed
---
