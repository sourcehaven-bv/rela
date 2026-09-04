---
id: RR-WEKTVV
type: review-response
title: Refusal misses a read-only role holding a world grant — IsPrivileged excludes Read
finding: |-
    VERIFIED, and sharper than the design review framed it: this is a DIRECT self-promotion into a world grant, not a chain.

    acl.RoleDef.IsPrivileged (policy.go:361-363) is `len(Create)>0 || len(Update)>0 || len(Delete)>0 || len(Permissions)>0` — Read is deliberately excluded per RR-LXI3NW/RR-UR0LJU, on the reasoning that 'a read-everything role is a visibility choice, not an escalation path'.

    TKT-DN37J2 INVALIDATES that reasoning. Once `read: [world:published]` exists, a read-only role can hold a non-default-world grant, and self-granting it IS the escalation — it is literally the threat docs/acl-security.md:77-93 describes.

    Counterexample defeating BOTH proposed refusal arms:

      role_relations:
        member-of: { requires_permission: delegate-admin }   # A1 arm => false (gated)
        owns:      { confers: viewer }                       # ungated
      roles:
        viewer: { read: [world:published] }                  # IsPrivileged => FALSE

    A2's checkUngatedRoleRelations (tier_a.go:76-78) does `if !ok || !isPrivileged(role) { continue }`, so it skips `owns` entirely. My proposed arm (b) (UngatedPrivilegedRoleRelationOpen) inherits that skip. A1 is false because membership is gated. The policy loads clean — and one `owns` edge write self-grants a published-world read.

    This also falsifies the plan's stated NECESSARY-CONDITION claim in dn37j2-plan.md §4.4, which the architect approved as fact in §8 Q5: 'an ungated privileged role-relation is a necessary condition of every A2-shaped chain'. It is not, because 'privileged' does not count world grants.

    Secondary (weaker, but real): Policy.AssertedRoles is read by computeGlobals (resolver.go:57-65) but scanned by neither predicate (RR-8ZOICR). A role reachable only via an IdP claim is invisible to both arms.
severity: critical
status: open
---
