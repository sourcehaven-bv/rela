---
id: FEAT-OF2ZOL
type: feature
title: Resolve principal to entity via principal_property + unique constraint
summary: ACL policy can resolve an authenticated principal (e.g. X-Forwarded-User email) to a user entity by property lookup, backed by a general unique property constraint.
description: Adds two coupled acl.yaml keys (user_entity_type + principal_property). At request start, if both are set, rela looks up the raw principal against the named property (e.g. persoon.email) and, on exactly one match, substitutes principal.User with that entity's ID for the rest of the request so membership/local-role walks operate from a real entity. Backed by a new general PropertyDef.Unique constraint (enforced in the entitymanager write path) which principal_property must reference. The data-entry middleware re-stamps ctx with the resolved ID + RawUser so the audit log records both the raw header and the resolved entity. Stacked on TKT-Z8A62F (configurable membership_relation).
priority: medium
status: proposed
---
