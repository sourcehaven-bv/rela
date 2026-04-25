---
id: RR-6NIDO
type: review-response
title: Plan frames change as cloning EntityDetail; return_to addition is a deliberate divergence
finding: EntityDetail.editEntity() (frontend/src/components/entity/EntityDetail.vue:214-220) does NOT add return_to — it relies on router.back() because the user came in via SPA history. DocumentView is deep-linkable, so router.back() from a form submit could leave the SPA entirely. The plan correctly adds return_to but presents it as 'consistency with rewritten links' rather than 'deliberate divergence from the EntityDetail pattern because the page is deep-linkable'. Misleading framing; could prompt a future maintainer to 'harmonise' the two paths and break the deep-link case.
severity: significant
resolution: 'Addressed in PLAN-2DJMH Approach section: rationale rewritten to explicitly call out the deliberate divergence from EntityDetail.vue''s no-return_to pattern, with the deep-linkability reason. Future maintainers will see the why, not just the what.'
status: addressed
---
