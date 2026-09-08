---
id: RR-ZI2PF7
type: review-response
title: NewDeclarative mutated the SharedBase-shared policy (concurrent map write)
finding: |-
    VERIFIED. normalizeWorldGrants writes `p.Roles[name] = role`, and NewDeclarative calls it. The path SharedBase.Assemble → resolveACL → buildACL(base.aclPolicy) → NewDeclarative means every assembly wrote to the *Policy pointer the base hands to EVERY tenant.

    That contradicts the SharedBase godoc (appbuild.go:1088-1095), which states flatly that 'Assembly only reads them' and explains why: a mutation is visible to every other consumer of the same base — cross-tenant in a multi-tenant host, cross-project on the desktop.

    The values converge (the split is idempotent), so this is not a wrong-answer bug. But two parallel Assemble calls are a concurrent map write on p.Roles, which is a hard panic, not a benign race. TestSharedBase_AssemblyDoesNotMutateSharedValues does not catch it — it compares metamodel counts and the project context, not the policy's role map.
severity: significant
resolution: |-
    normalizeWorldGrants now early-continues on any role with no `world:` token still in Read (new `roleHasWorldToken` helper), so an ALREADY-SPLIT policy — which every Validate-loaded one is — never reaches the map write at all. The shared-policy path becomes a pure read.

    Chosen over cloning the policy: a clone would silently drop the split for the caller's own policy, which is exactly the silent-misconfiguration shape RR-LWE222 was about. The comment at the early-continue says it is load-bearing for shared policies rather than an optimization, so it does not get 'simplified' away.
status: addressed
---
