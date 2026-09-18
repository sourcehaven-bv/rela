---
id: RR-7Z3SFC
type: review-response
title: Pre-linked edge could be silently dropped between relations.value and the create payload (cards widget, wizard prune)
finding: 'The prefill writes the peer into relations.value, but relations.value is NOT the payload. Two filters sit between them: pruneWizardHiddenRelations drops a relation whose wizard step is not active, and the cardRelations exclusion drops card-managed relations because those are supposed to arrive via pendingCardChanges, which the prefill never writes. Either one eats a prefilled edge, and because linkAs=to is handled entirely by the payload there is no post-create call to notice. Result: entity created, no edge, no error, user returns to the originating entity and the section is still empty — indistinguishable from a stale page. Config-load validation cannot catch it: validateSectionCreate checks only that a form exists for the type, never how that form renders the relation.'
severity: critical
resolution: 'Added a post-condition check after relationsPayload is assembled: when linkAs is ''to'', the pre-linked peer must actually be present in the payload, or the create is refused with a message naming the relation and telling the operator to check the section config against the form. Refusing beats creating an unlinked entity, because the user cannot tell the latter happened. Checked rather than prevented deliberately: routing the prefill into pendingCardChanges would duplicate the card widget''s bookkeeping, and forcing a wizard step active would override an author''s visible_when. Covered by two tests (cards-widget refusal plus an ordinary-case positive), mutation-verified.'
status: addressed
---

## Finding

The prefill writes the peer into `relations.value` — but **`relations.value` is
not the payload.** Two filters sit between them:

1. `pruneWizardHiddenRelations` (`DynamicForm.vue:1272-1278`) drops a relation whose
wizard step is not active. The prefill runs in `initializeDefaults` with no
knowledge of steps.
2. The `cardRelations` exclusion (`:1409-1420`) drops card-managed relations, because
those are supposed to arrive via `pendingCardChanges` — which the prefill never
writes.

Either one eats a prefilled edge. And because `linkAs: to` is handled *entirely*
by the payload, there is no post-create call to notice: the entity is created,
no edge is written, no error is raised, and the user returns to the originating
entity to find the section still empty — indistinguishable from a stale page.

This is the same failure RR-8SP2UG and AC11 exist to prevent, reached from the
other direction.

Config-load validation cannot catch it: `validateSectionCreate` checks only that
*a* form exists for the type, never how that form renders the relation.

## Resolution

A post-condition check after `relationsPayload` is assembled: when `linkAs` is
`to`, the pre-linked peer must actually be present in the payload, or the create
is refused with a message naming the relation and pointing the operator at the
section config.

**Refusing beats creating an unlinked entity**, because the user cannot tell the
latter happened.

Checked rather than prevented, deliberately: routing the prefill into
`pendingCardChanges` would duplicate the card widget's own bookkeeping, and
forcing a wizard step active would override an author's `visible_when`. Failing
loudly puts the problem in front of the only person who can fix it.

Two tests: the cards-widget refusal, and an ordinary-case positive so the check
cannot refuse a working form. Mutation-verified — neutering the condition fails
the first.
