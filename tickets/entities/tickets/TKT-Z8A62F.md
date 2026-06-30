---
id: TKT-Z8A62F
type: ticket
title: 'ACL: make the membership relation (member-of) configurable via membership_relation: in acl.yaml'
kind: enhancement
priority: medium
effort: s
status: done
---

Promote the hard-coded `"member-of"` literal in `internal/acl/resolver.go` to a
configurable `Policy.MembershipRelation` field (`membership_relation:` in
`acl.yaml`). Default stays `"member-of"` — existing policies are unaffected.

## Why

The ACL resolver maps principal → group membership → role by walking `member-of`
edges. Today the relation name is hard-coded. Operators who already model a
semantic "who has which role/group" relation in their metamodel (e.g.
`heeft_rol` in a Dutch-language ISMS) must maintain a second, parallel
`member-of` edge system, causing shadow administration: two places claiming "who
is CISO" that drift, `_actions` UI on the wrong edge, audit trail split from
domain history, dual migration pipelines. One configurable key gives a single
source of truth.

## Change

- **`internal/acl/policy.go`**: add `MembershipRelation string yaml:"membership_relation"`
to `Policy`; add `"membership_relation"` to `knownPolicyKeys`. Default to
`"member-of"` when empty (in `Validate`). Add two hardening warnings: membership
relation set but assignments empty; membership relation != default but no
`role_relations.<rel>.requires_permission` gate (escalation foot-gun).
- **`internal/acl/resolver.go:65`**: `OutgoingRelations(ctx, n, "member-of")` →
`OutgoingRelations(ctx, n, r.d.policy.MembershipRelation)`. Update the
`computeGlobals` / `walkMembers` docstrings.
- **Docstrings**: `Policy` struct (new field), `RoleRelationDef` (rename the
"Escalation risk for the member-of relation" text to "the configured membership
relation", note member-of is the default).
- **Docs**: `docs/acl-overview.md` (note the key is optional + default),
`docs/security.md` (extend "Hardening member-of" to cover any configured
membership relation), `docs/concepts.md` if it introduces member-of.

## Out of scope

- Multiple membership relations at once (one field, not a list — list is still
backwards-compatible if a use case surfaces).
- `_actions` UI changes (relation name varies per project, wire shape identical).
- Migration tooling (default stays member-of; operators who switch manage it).

## Acceptance

1. `acl.yaml` with `membership_relation: heeft_rol` + assignments mapping a group
to a role: a principal with a `heeft_rol` edge to that group gets the role,
attributed `Source{Kind: SourceGroup, Group: <group>}`.
2. Same policy without the `heeft_rol` edge: principal gets no group role.
3. `acl.yaml` without `membership_relation:` behaves identically to today; all
existing `resolver_test.go` cases pass unchanged.
4. `membership_relation: heeft_rol` + only a `member-of` edge in the graph: that
edge is no longer followed (no role granted).
5. `Policy.Validate` emits the hardening warning when `membership_relation:` is
set to a non-default value without a `requires_permission` gate.
6. Docstrings + docs mention the new field and its default.
