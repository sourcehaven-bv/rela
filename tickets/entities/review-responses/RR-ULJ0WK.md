---
id: RR-ULJ0WK
type: review-response
title: _actions.create does not exist on the entity response; AC1 rests on a missing field
finding: 'affordances.go:131-135 splits the verbs: perItemVerbs = update/delete/rename, perCollectionVerbs = create. computeActions (:144) builds the per-entity map from perItemVerbs only, so an entity GET response has no `create` key at all. EntityDetail.vue:271-272 corroborates — canUpdate/canDelete exist, canCreate does not. The ticket''s Flow step 1, AC1, and the stated mitigation for the top-listed risk (affordance/write divergence) all specified rendering Duplicate when _actions.create is true, so the single mitigation named for the top risk was unimplementable as written.'
severity: critical
resolution: AC1 and Flow step 1 rewritten to use the sidebar inline_create map instead of the nonexistent _actions.create. Also removed the planned _duplicate affordance key from scope, since inline_create supplies the signal.
status: addressed
---

## Resolution

Use `SidebarData.inline_create` (`responses.go:803-821`, served
`views_handler.go:287`, read via `schema.ts:471` `inlineCreateFormFor`).

Its presence already means "the principal may create this type AND a create form
resolves for it" — both conditions Duplicate needs — and it ships the resolved
form id, so the client does no permission arithmetic and no form lookup. It is
documented as a UI hint, never authorization; the create re-authorizes.

This also removes the need for a new `_duplicate` affordance key, and with it
the "omitted, never empty" wire constraint and the `_copies` pattern-matching.
Net scope reduction.
