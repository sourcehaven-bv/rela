---
id: RR-S7A16Q
type: review-response
title: Chained escalation via ungated non-membership role-relation is warning-blind
finding: 'Policy.MembershipSelfPromotionOpen() returns false when the membership relation carries requires_permission, but the gate can be circumvented in two writes if ANOTHER role-relation is ungated and confers a role holding the gating permission (A2''s domain): write the ungated edge to gain delegate-membership, then write the membership edge. The boot warning evaluates only the A1 predicate, so this chained shape boots silently; `rela acl audit` A2 still flags it at High.'
severity: minor
reason: Out of scope for TKT-T31NKT (behaviour-identical extraction of the A1 predicate + warning on it). The chained path is already reported by the audit's A2-ungated-role-relations finding, so it is not silent — only the boot warning misses it. Flagged to the architect because TKT-DN37J2's planned load refusal keys on the same shared predicate and would inherit the blind spot; deciding whether the refusal must also consider A2-shaped chains is a design decision for that ticket.
status: deferred
---
