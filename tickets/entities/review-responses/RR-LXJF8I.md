---
id: RR-LXJF8I
type: review-response
title: 'grantsVerb widened write authority: a state grant authorized the default face'
finding: |-
    VERIFIED by execution before fixing. PR-A's first cut changed grantsVerb to `grantTypeOf(t) == target`, so `update: ["page@draft"]` returned TRUE from grantsVerb(role, OpUpdate, "page").

    Why that is a widening and not a convenience: grantsVerb IS the live write-authorization path (decideFromAttrs, authz_write.go:103, which knows the entity type but not yet which FACE is being written). GrantsVerbOnState — the face-granular check — had ZERO production callers. So the narrowest grant the new syntax offers would have authorized MORE than the operator wrote and more than they had before: a grant must never widen by being made more specific.

    Also verified two consequences of an entity type legitimately containing '@' (ValidateSchemaName is a blocklist that permits it):
    - a grant on a type named `a@b` STOPPED matching it (regression)
    - a grant on `a@b` STARTED matching a different type named `a` (cross-type escalation)
severity: critical
resolution: |-
    Two-part fix.

    1. grantsVerb now SKIPS state-shaped entries entirely (`isStateGrant` -> continue) and compares bare entries literally as before. A state grant authorizes nothing until the write path is face-aware — the fail-closed direction, costing an operator a denied write they will ask about rather than a silent over-permit nobody notices. The godoc states this is half of a two-part change and names GrantsVerbOnState as the other half.

    2. Entity type names may no longer contain '@' (metamodel/loader.go, entity-type-specific rather than in the shared ValidateSchemaName, since property names are not grant subjects). This closes the ambiguity at the root in both directions.

    grantForRole (access.go, backing `rela acl who-can` / `rela acl map`) got the same skip, so the operator-facing reports agree with the runtime.
status: addressed
---
