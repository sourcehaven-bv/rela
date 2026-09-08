---
id: RR-2ITZ84
type: review-response
title: 'Affordance when: bound the unknown placeholder as a real identity'
finding: 'internal/affordances/bindings.go bound is_current_user/has_current_user (and current_user.id) from principal.From(ctx).User, which is the literal "unknown" placeholder for an unstamped or identity-less request. Affordance when: clauses gate visible:/readonly:/actions, so `is_current_user(entity.owner)` would grant to an anonymous caller for an entity whose owner literally holds "unknown". Reachable through the `everyone` role. The query path refuses exactly this state (ErrNoCurrentUser); the affordance path did not, while both were documented as one implementation. Raised independently by both the security and architecture reviewers.'
severity: significant
resolution: bindingContext.identity() maps principal.Unknown to ""; PolicyResolver.passes refuses any grant whose clause reads the current user (predicatefns.RequiresCurrentUser) when the identity is empty; predicatefns.CurrentUserBindings("") never matches as defence in depth. Pinned by TestAffordances_CurrentUserSugar_UnknownPlaceholderNeverMatches (everyone role, unknown and unstamped, all three spellings, including an empty owner) and TestCurrentUserBindings_EmptyIdentityNeverMatches.
status: addressed
---
