---
id: RR-G3Y70
type: review-response
title: Negative literals cannot be compared against Int/Date fields
finding: entity.count > -5 fails compile with 'unary minus is not allowed' (predicate grammar, pre-existing). This change makes signed-integer properties first-class, so the inability to write entity.balance > -100 is now a user-visible gap in the shipped feature. Fix is out of Phase-1 scope (grammar change to allow negative numeric literals). Document in ticket/docs; address in a follow-up.
severity: minor
reason: Pre-existing grammar limitation (unary minus disallowed by the predicate walker), not introduced by this change. Allowing negative numeric literals is a grammar/walker change orthogonal to the typed-value work; scoping it into Phase 1 would widen the walker's allow-list surface. Deferred to a dedicated follow-up ticket. Workaround exists (compare against 0 or reformulate). Will note the gap in docs when the CLI --filter docs land in Phase 2.
status: deferred
---
