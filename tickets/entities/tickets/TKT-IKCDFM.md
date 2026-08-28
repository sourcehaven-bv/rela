---
id: TKT-IKCDFM
type: ticket
title: 'aclaudit: model asserted_role_assignments as a privileged-role path (new A-tier check)'
kind: enhancement
priority: medium
effort: s
status: backlog
---

Follow-up from TKT-T31NKT code review (RR-8ZOICR, deferred there as
correctly-out-of-scope for the membership predicate).

`asserted_role_assignments` — roles conferred from verified IdP claims — is a
privileged-role path that no audit tier currently models: not A1 (membership
edge), not A2 (ungated role-relations), not the shared
`acl.Policy.MembershipSelfPromotionOpen` predicate. The trust story differs from
graph edges (claims come from the identity proxy, not from writable graph
state), so a claim-mapped privileged role is NOT self-promotion in the A1 sense
— the check must reason about what an attacker who controls claims (or a
misconfigured proxy mapping) gains, and at minimum surface visibility-level
findings ("these roles are conferred by claims, these of them are privileged")
so the operator sees the full role-conferral surface in one audit run.

Scope: audit-only (aclaudit tier A or a new informational tier); no enforcement
change. Keep the single-definition discipline: reuse `RoleDef.IsPrivileged`.
Becomes more relevant once worlds ship — a claim-conferred role holding a
non-default-world read grant is part of the TKT-DN37J2 refusal discussion
(RR-S7A16Q note there), but this ticket stands alone as audit coverage.
