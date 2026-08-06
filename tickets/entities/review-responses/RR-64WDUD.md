---
id: RR-64WDUD
type: review-response
title: system:provisioner minimal grant contradicts 'automations add the groups' — cascade authorizes as provisioner
finding: 'The plan''s central division-of-labor is: rela creates the stub, the operator''s on-create automations add group memberships / roles. But the on-create cascade runs on the SAME ctx the create ran on (manager.go:487 Cascade.Process(ctx, ...)), and the plan runs the create under provCtx = With(ctx, system:provisioner). So the cascade''s nested writes authorize as system:provisioner (verified: gated() strips ACL-bypass but not the principal; cascade nested writes use principal.From(ctx)). system:provisioner is granted ONLY create:[user_entity_type]. An on-create automation that creates a member-of relation needs relation-create rights it does not have -> the cascade errors -> CreateEntity returns an error (manager.go:494) -> the whole provision fails. So AC4 (''on-create automation adds a member-of edge fires on the stub'') is directly incompatible with the minimal-grant security posture. This is a design contradiction, not a bug: either the provisioner grant widens to cover the relation/entity types the operator''s automations write (blast radius + operator responsibility), or provisioning cannot rely on automations for the domain half.'
severity: critical
resolution: 'DEFERRED to the provision ticket (parked in .ignored/provision-unmatched-principal-design.md). Decided direction: BARE STUB ONLY — rela creates the minimal user entity and stops, no group membership at provision time, so system:provisioner stays create-user-only and the cascade-authz contradiction cannot arise. Not relevant to this ticket, which is reject-only (no provision, no system principal, no cascade).'
status: deferred
---

## Finding

The plan's headline: rela creates the stub, the operator's **on-create
automations** add groups/roles. Traced against code, this contradicts the
minimal-grant posture.

The on-create cascade runs on the create's ctx (`manager.go:487`
`Cascade.Process(ctx, …)`), and the plan runs the create under `provCtx =
With(ctx, system:provisioner)`. So the cascade's nested writes authorize as
`system:provisioner` (`gated()` at manager.go:103 strips ACL *bypass*, not the
principal; nested writes use `principal.From(ctx)`).

`system:provisioner` is granted **only** `create: [user_entity_type]`. An
on-create automation that creates a `member-of` relation needs relation-create
rights it lacks → the cascade errors → `CreateEntity` returns an error
(manager.go:494) → the whole provision fails.

So **AC4 is incompatible with the minimal grant.** The elegant division of labor
cannot work as designed.

## Resolution options (decision needed)

1. **Widen the provisioner grant** to cover the relation/entity types the
operator's on-create automations write. Larger blast radius; must be an
explicit, documented operator responsibility with its own AC. The grant is no
longer "minimal."
2. **rela stamps a default group/role itself** (declaratively, from a policy
key) rather than delegating to automations — keeps the grant minimal but moves
the domain decision back into rela, which the plan deliberately avoided.
3. **Provision the bare stub only; no group membership at provision time.** The
operator reconciles groups out-of-band (webhook, a scheduled job, manual).
Simplest grant, but the stub is inert until something else acts — which partly
defeats the point.

Cannot run the cascade elevated (elevation-doesn't-propagate is load-bearing,
manager.go:492, TKT-D8T148).
