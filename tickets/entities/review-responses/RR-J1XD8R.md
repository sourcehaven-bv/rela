---
id: RR-J1XD8R
type: review-response
title: The identity-disagreement refusal rendered advice for a different problem
finding: nextActionRequestScope wrapped the stamp-vs-principal disagreement in ErrIdentityRequired, so the handler emitted next_action_identity_required with the hint 'configure an identity source' — for a request that has TWO identities, not none. The test only asserted errors.Is, so nothing pinned the rendered message.
severity: minor
resolution: New sentinel nextaction.ErrIdentityConflict; the scope binder returns it directly; writeNextActionError maps it to next_action_identity_conflict with its own title and hint, echoing neither identity. TestNextActionRequestScope asserts the conflict is NOT an ErrIdentityRequired; TestNextAction_IdentityConflictIsNamed pins the HTTP code and that no identity is echoed.
status: addressed
---
