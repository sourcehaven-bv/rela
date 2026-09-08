---
id: RR-H98VX0
type: review-response
title: 'ACL provisioner migration DESTROYS a state-shaped create: grant'
finding: |-
    VERIFIED, and it is destructive, not merely blind. Not mentioned anywhere in the plan.

    internal/migration/acl_provisioner_grant.go:
    - hasCreateTarget (:264-277) walks the `create:` YAML SequenceNode comparing `t.Value == "*" || t.Value == userType`.
    - On false, ensureRoleGrantsCreate (:243-259) calls SetMapNode(def, "create", createList(userType)).
    - createList returns a ONE-ELEMENT sequence, and SetMapNode REPLACES the value node outright (`node.Content[i+1] = value`).

    So `create: ["person@draft", "ticket"]` becomes `create: ["person"]`. The state-shaped grant is destroyed AND the unrelated `ticket` grant is collateral damage. Silent, on a routine migration run.

    This is a YAML-node access path that never mentions RoleDef, so a struct-field grep could not see it — the blind-spot class dn37j2-plan.md §9 flagged.

    Sibling checked: acl_scheduler_grant.go:114-148 (hasReadTargets) is LENGTH-only, so a world grant counts as a read target and it does not fire. Lower risk, but it writes ["*"] when it does fire — audit it in the same pass.

    FIX: teach hasCreateTarget the joined form (compare on the type half). Regression test: a migration over a policy carrying `type@face` and `world:` grants must be a no-op.
severity: critical
status: open
---
