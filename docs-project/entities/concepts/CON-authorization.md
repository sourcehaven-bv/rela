---
id: CON-authorization
type: concept
title: "Authorization (ACL)"
summary: "Principal-aware authorization for entity and relation writes — who can write what, against the graph's structure"
---

rela's authorization (ACL) system gates writes against a YAML-declared
policy and the graph itself. It answers two questions on every write:
**who is the principal**, and **what role(s) does that principal hold
for the entity being written**? When the answer's role set contains
write permission for the entity's type, the write is allowed; otherwise
it's denied with a structured reason that names the rule.

This concept introduces the vocabulary; see the linked guides for an
operator's overview and the security hardening notes.

## Vocabulary

**Principal.** Who is making the write. Stamped onto the request
context by the entry-point middleware. The principal has a
`User` (the identity) and a `Tool` (data-entry, MCP, scheduler, CLI).
An unstamped principal (`User == "" / "unknown"`) is treated as
anonymous and is denied; ACL does not silently allow.

**Subject.** What is being written, as a sealed sum:

- `EntitySubject{ID, Type}` — an entity Create / Update / Delete /
  Rename. ID may be empty on Create.
- `RelationSubject{Type, FromType, FromID}` — a relation Create or
  Delete. The "to" side is intentionally absent; v1 grants relation
  writes by source type only.

A nil Subject is a programmer error and panics — the call site is
buggy.

**Policy.** The YAML at `acl.yaml`: roles (with `read`, `write`,
`permissions`, optional `fields`/`relations`/`groups`), assignments
(`user → role`), role-relations (relation types whose presence
confers a role), and `inherit_roles_through` (relation types whose
ancestry inherits role grants).

**Graph.** The store-backed adapter that answers `HasEdge` and
`OutgoingRelations` for the resolver's traversal. Production wires
it to `acl.NewStoreGraph(s)`; tests use `acl.NullGraph{}` for "no
edges anywhere."

**Request.** A per-HTTP-request resolver scope. Constructed by
`Declarative.ForPrincipal(p)`. Caches one `member-of` closure walk
and per-entity attribution sets so a list response amortises one
graph walk across every per-entity probe.

**Source.** How a role landed in a principal's effective set:

- `Global` — direct assignment to the principal.
- `Group` — assignment to a group the principal is in (via
  `member-of` closure).
- `Local` — a role-relation edge from the principal to the entity.
- `LocalViaGroup` — role-relation edge from a group the principal
  is in.
- `LocalViaAncestor` — role-relation edge from the principal to an
  entity that contains the target (via `inherit_roles_through`).
- `LocalViaGroupAndAncestor` — both of the above combined.

Provenance matters: a denied write's audit row records which Source
contributed which role, so an operator can trace "who could and
couldn't do this" without reading the policy and graph by hand.

**Decision.** What `AuthorizeWrite` returns:

- `Allow bool`
- `RuleKind string` — the gate that fired (`role-grant`,
  `delegate-permission`, `read-only`).
- `RuleID string` — the specific rule within RuleKind (role name,
  permission name).
- `Reason string` — human-readable, never carries raw policy data.
- `Attributions []RoleAttribution` — the full (role, source) set
  the resolver considered, recorded server-side in the audit log;
  the wire 403 body deliberately omits this so deny responses don't
  leak topology.

## What ACL does not do

- **Read-side gating on list queries.** Deferred — v1 gates writes
  and per-entity reads via affordances; list filtering lands in a
  follow-up.
- **Authentication.** rela-server has no auth layer; principals are
  stamped by the calling tool. See the security guide.
- **Per-link verdicts.** Relation writes are gated by source type
  only; asymmetric grants like "may create editor-of edges only to
  projects" are deferred until there's a concrete need.

## Where to read next

- [GUIDE-acl-overview] — operator's overview with sequence + concept diagrams.
- [GUIDE-acl-security] — member-of hardening, why nil Subject panics,
  why malformed `acl.yaml` fails boot.
