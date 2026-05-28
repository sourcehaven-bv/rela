---
id: RR-BHSM
type: review-response
title: Rename default role to everyone; share name via acl constant
finding: 'Crit round 3 (Jeroen): the implicit role name ''default'' feels off; prefer ''everyone''. And the name was duplicated between acl.Declarative and the affordances resolver, kept in sync manually. Asked to rename in both and share the constant.'
severity: minor
resolution: Added acl.EveryoneRole = "everyone" in internal/acl/policy.go as the single source of truth. acl.Declarative.effectiveRoles and affordances.globalRoles both reference it. Updated all tests and docs/security.md. Doc note added that anonymous/authenticated built-ins would join it there alongside a future auth layer.
status: addressed
---
