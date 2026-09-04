---
id: RR-E0L2PG
type: review-response
title: 'A2 did not share the refusal predicate: audit reported clean on a policy the server refuses'
finding: |-
    VERIFIED. Ruling 8 requires, verbatim, that `checkUngatedRoleRelations` call the extracted predicate 'so the linter and the boot refusal cannot diverge (TKT-T31NKT's shared-predicate property, extended)'. PR-A's first cut did not do that — A2 kept its own inline loop over p.RoleRelations using isPrivileged(role), while the refusal used UngatedPrivilegedRoleRelationOpen + roleIsEscalationRelevant.

    They already disagreed by exactly the world term. A policy refused at load for `owns → viewer{read: [world:published]}` produced NO A2 finding — so `rela acl audit` reported clean on a policy the server refuses to boot. That is the worst possible operator experience here, because the refusal's own error text tells them to run that audit for the full picture.
severity: significant
resolution: 'Extracted `Policy.RoleRelationEscalates(relType)` as the per-relation predicate and had BOTH consume it: aclaudit''s checkUngatedRoleRelations and UngatedPrivilegedRoleRelationOpen (the refusal''s second arm). Verified by running the audit over the counterexample policy — it now emits `A2-ungated-role-relation [high] owns`, matching the refusal.'
status: addressed
---
