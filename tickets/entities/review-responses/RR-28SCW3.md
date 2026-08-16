---
id: RR-28SCW3
type: review-response
title: 'provision: system:provisioner minimal grant vs. ''automations add groups'' — cascade authorizes as provisioner'
finding: If provision runs the stub create under provCtx = With(ctx, system:provisioner), the synchronous on-create cascade (manager.go Cascade.Process runs on the create's ctx) authorizes its nested writes as system:provisioner (gated() strips ACL-bypass but not the principal). system:provisioner is granted only create:[user_entity_type]. An on-create automation that creates a member-of relation needs relation-create rights it lacks -> the cascade errors -> the whole provision fails. So 'automations add the groups' is incompatible with a minimal provisioner grant.
severity: significant
resolution: 'Decided direction (with the user): BARE STUB ONLY. rela creates the minimal user entity and stops — no group membership at provision time — so system:provisioner stays create-user-only and the cascade-authz contradiction cannot arise. Asserted JWT roles already apply independently of the entity, so the stub is not inert. Groups arrive later (webhook/reconcile/admin). This is the design; the ticket (TKT-ANUJDS) implements it. Re-recorded here after being deleted when TKT-0C3II2 narrowed to reject-only.'
status: addressed
---

## Finding

Running the stub create under `provCtx = With(ctx, system:provisioner)` makes
the synchronous on-create cascade authorize its nested writes as
`system:provisioner` (`gated()` strips ACL bypass, not the principal). That
principal has only `create: [user_entity_type]`, so an automation adding a
`member-of` edge fails and takes the whole provision with it.

## Resolution

**Bare stub only** (decided with the user). No group membership at provision;
`system:provisioner` stays create-user-only. Asserted JWT roles already apply,
so the stub isn't inert. Groups arrive later, out of scope. Implemented by
TKT-ANUJDS. (Re-recorded after deletion when TKT-0C3II2 became reject-only.)
