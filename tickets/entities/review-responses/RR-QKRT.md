---
id: RR-QKRT
type: review-response
title: has_role must take the entity for local (entity-scoped) roles like owner
finding: Draft declared has_role(user, role_name) bool — global-only. But the ACL four-layer model (.ignored/acl-design.md) has local roles (owner, editor, reviewer) conferred per-entity by role_relations edges (alice --owner-of--> TKT-001). 'Is current_user an owner?' is meaningless without 'owner of WHAT.' A global-only has_role could never express the canonical affordance 'owners may edit internal notes.' Raised by Jeroen in crit round 1.
severity: critical
resolution: 'has_role is now 3-arg: has_role(current_user, entity, role_name) bool. The resolver computes the (principal, entity) effective role set (global roles ∪ direct local roles conferred on that entity) once per FieldVerdicts call against the snapshot. Local-role grant blocks participate in the same per-role-per-type union (DR-S3), naturally entity-scoped. Inherited local roles via inherit_roles_through are deferred to ACL v1; v1 resolves direct local roles only.'
status: addressed
---
