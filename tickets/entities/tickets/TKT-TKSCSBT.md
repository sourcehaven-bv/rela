---
id: TKT-TKSCSBT
type: ticket
title: 'Conditional grant edges: when: on role_relations and membership'
kind: enhancement
priority: high
effort: l
started: "2026-11-16"
completed: "2026-11-17"
status: done
---

## Description

Let `acl.yaml` declare a condition on a grant edge, so the edge counts only
while the condition holds. Example: an external auditor gets read access for six
weeks through a `heeft_rol` edge with `valid_until: 2026-12-31` and

```yaml
role_relations:
  heeft_rol:
    confers: auditor
    requires_permission: delegate-roles
    when: "edge.valid_until >= today"
```

The same `when:` on the membership relation's entry filters the member walk.

Access ends at the first request after the date. No scheduled job runs, and
identity is not cached across requests.

## Acceptance

- `when:` parses as an AND of `edge.<prop> (==|>=|<=) (<literal>|now|today)`;
anything else fails the policy load.
- Load fails when a referenced property is not declared on the relation, or
an ordered comparison targets a type whose byte order is not its time order.
- The condition lowers to a new `store.RelationPredicate.EdgeProps`
dimension, implemented in naive, pgstore and sqlitestore, with conformance and
differential tests.
- Every grant-edge path applies it: membership walk, per-entity probe, primed
batch, world grant, `ReadQuery`, `LocalRolePredicate`.
- Fail closed: missing, empty, list or unparseable date means no grant.
- `now`/`today` come from the read-time-now work (gap item 3a); no second clock.
- `rela acl audit` reports conditional grants.
- docs-project ACL guide explains expiry semantics.

## Research

See `.ignored/gap10-research.md` (summarised in the linked research entity).
