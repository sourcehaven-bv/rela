---
id: RR-U41F3D
type: review-response
title: A baseline denial is carve-out-able by any scope, undocumented
finding: verbCeiling.except is checked ahead of both allow and deny (ceilingcompile.go:71-76), so a scope grant naming a capability re-opens it even when a baseline explicitly denied it. There is no way to mark a baseline denial un-carve-out-able. The reviewer confirmed this cannot escalate past the acting user (the result still intersects user_grants at filterTypes/grantsVerb) and could not construct an escalation — so it is a documentation gap, not a hole. A12 only fires for the inverse case (a scope re-opening what NO baseline closes), so nothing surfaces this to an operator.
severity: minor
resolution: Documented rather than changed, because the behaviour is the intended OAuth semantic and an un-carve-out-able marker would introduce a precedence rule the design deliberately has none of. Added a 'A baseline denial is a default, not a floor' section to the ScopeGrant godoc (internal/acl/ceiling.go) and the same guidance in operator prose in GUIDE-acl-overview.md. The safety argument is unchanged and pinned by TestCeiling_NeverGrantsPastTheUser and TestScopes_MoreScopesNeverNarrows.
status: addressed
---

## Resolution

Documented in two places rather than changed, because the behaviour is the
intended OAuth semantic ("scopes widen only WITHIN the ceiling") and adding an
un-carve-out-able marker would introduce a precedence rule the design
deliberately has none of.

- `ScopeGrant` godoc (`internal/acl/ceiling.go`) gains a "A baseline denial is a
default, not a floor" section: any scope naming a capability re-opens it; the
floor is the acting user, not the baseline; if a capability must never reach a
client class, do not write a scope grant naming it.
- The same guidance in operator-facing prose in `GUIDE-acl-overview.md`, since
this is a mental-model question an operator hits before a developer does.

The safety argument is unchanged and already pinned by
`TestCeiling_NeverGrantsPastTheUser` (entitymanager) and
`TestScopes_MoreScopesNeverNarrows` (acl).
