---
id: TKT-CXQEV0
type: ticket
title: 'related(): traverse incoming edges (filter B on properties of A where A -> B)'
kind: enhancement
priority: medium
effort: xl
status: done
---

## Description

`related()` (TKT-RELTRV) only follows edges outgoing from the entity the
condition is about. Given `A -[r]-> B`, a view on A can filter on B's
properties, but a view on B cannot filter on A's.

Example: a view of features that have at least one ticket implementing them with
`status == 'in-progress'`. `ResolveTraversalTarget` refuses this with `related:
relation "implements" does not start from "feature"`
(`internal/predicatefns/traversal.go:80`).

Note: TKT-RELTRV is unwired. No production code calls `ValidateTraversals`,
`GateTraversal` or `Bindings.SetTraversal`. A `related()` condition compiles on
every surface but fails at evaluation, and only index derivation
(`queryplan.TraversalIndexSpecs`) reads it. The wiring was recorded as a
follow-up in REV-RELTRV but never filed.

The store layer can already express it: `store.EndpointPredicate` has both
`HasInbound` and `HasOutbound`. What is missing:

- a way to write an incoming hop in `related()`;
- direction in `predicate.TraversalSpec` and `acl.TraversalHop`;
- resolution against `RelationDef.From` (instead of `To`) for an incoming hop,
including the `type =` ascription rule for a union `From`;
- lowering to an inbound `RelationPredicate` in pgstore, sqlitestore and
graphquerynaive, with the same per-hop ACL gate;
- index derivation in `queryplan` for the traversed-FROM type.

## Acceptance criteria

- A condition on B can filter on properties of entities A that point at B
through relation r, and it pushes down to SQL (no per-row Go evaluation).
- Incoming and outgoing hops can be chained.
- The ACL rules of TKT-RELTRV (per-hop row gate, field gate, no inheritance
expansion, face refusal) apply unchanged to an incoming hop.
- All three backends return the same rows (storetest).
