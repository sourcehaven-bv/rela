---
id: TKT-0C3II2
type: ticket
title: Auto-provision a user entity for an unmatched verified principal
kind: enhancement
priority: medium
effort: m
status: backlog
---

Follow-up to TKT-RP3X3Q. Decide and implement what happens when a
cryptographically verified principal has **no matching user entity** in the
graph — today it degrades to an anonymous-but-role-bearing principal.

## Context

TKT-RP3X3Q makes asserted roles from a verified JWT grantable in `acl.yaml`.
That decoupled role-granting from graph membership, which raised the question:
what should a verified principal with valid roles but no `user_entity_type`
entity be able to do?

**Decided for TKT-RP3X3Q (interim):** unmatched principal → anonymous. The
request stays authenticated and keeps its asserted roles; it simply has no
resolved user entity, so anything keyed on that entity (local roles via
`role_relations`, ancestry via `inherit_roles_through`, `principal_property`
lookups) does not apply. This is a deliberate interim position, not the end
state.

## Why a follow-up is needed

Behind an OIDC proxy with SSO JIT provisioning, this is not an edge case — it is
the **first request of every new user**. Pratique will happily mint a valid
assertion for a freshly-provisioned member (`roles: []` or a default role) who
has never existed in rela. The anonymous fallback means such a user gets
asserted-role grants but silently loses every graph-derived affordance, with no
signal to the operator that a person is transacting without an entity.

Consequences worth designing away:

- **Audit attribution degrades.** Writes are attributed to the raw subject with
no entity to join against, so per-user history is harder to reconstruct.
- **Silent, not loud.** Nothing surfaces "N verified principals have no user
entity" — the operator finds out when someone reports missing permissions.
- **Drift.** Users provisioned in the IdP and users present in rela diverge
over time with no reconciliation.

## Questions to answer

1. **Auto-create, or not?** Options: create a `user_entity_type` entity on first
verified request; create lazily only on first *write*; never create but surface
an operator report; or an explicit `rela user import` reconciliation command.
2. **If auto-created, with what?** `sub` is the stable key, but `email`,
`org_id`/`org_slug` are also on the assertion (see TKT-RP3X3Q for the full
verified claim set). Which become properties? Which relations, if any?
3. **Who is the author?** An auto-created entity is a write on the read path —
which violates the project rule against user-supplied work on reads, and needs a
system principal, an audit story, and a decision about whether it runs inside a
`Tx`.
4. **Idempotency and races.** Concurrent first requests from the same subject
must not create duplicates. `if_exists: skip` semantics, or a unique constraint
on the principal property.
5. **Should it be opt-in?** A deployment that treats the graph as the authority
on who exists may want unmatched principals rejected outright rather than
auto-created. Likely a policy key with a fail-closed default.
6. **Interaction with `isUnstamped`.** The current fail-closed gate
(`acl/request.go:189`) checks User/Tool only. Any change here touches a security
gate and needs its own review.

## Notes

- Related: the org-enforcement follow-up also carved out of TKT-RP3X3Q. Both
concern "what does a verified multi-tenant identity mean to the graph", and may
want a shared decision record.
- `create_entity` automations with `if_exists: skip` are the existing in-repo
precedent for idempotent entity creation.
