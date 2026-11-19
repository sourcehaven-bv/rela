---
id: DEC-TDEF3GD
type: decision
title: As-of reads are gated by today's ACL; field visibility follows history's fail-closed rules
context: An as-of read needs an authorization rule. The ACL policy (acl.yaml) is operator config and is not versioned in the store, so the policy at the instant cannot be reconstructed. Identity (roles, memberships) is resolved once at the request edge from the live graph.
consequences: A principal who lost access cannot read the past through history; an auditor granted access today can. The row gate reads today's edges, so a row that still exists passes only if today's gate admits it; an entity deleted since shows only to a reader with a global read role. A field condition on the row's own values is judged at the instant. Field visibility uses the history endpoint's rules (TKT-73C6B2), so a field shown only through a role relation or a related(...) condition is hidden in a past read, even to someone who sees it live. Policy changes apply retroactively to as-of reads.
date: "2026-11-16"
status: accepted
---

## Decision

An as-of read (`?as_of=`) is authorized by TODAY's ACL policy and TODAY's
identity of the principal (roles and memberships, resolved at the request edge
from the live graph). The row gate reads today's edges: the role relation, its
endpoint rows and the inherit-through edges. Access lost since the instant
therefore fails closed. World grants are today's.

The project owner chose to fail closed: a row that still exists must pass
today's read gate. The gate reads no value of the row itself, only today's
edges and endpoint rows by the row's id and face, so this holds by
construction. An entity deleted since the instant has no edges today. A
reader who reaches it only through a role relation does not see it; a reader
with a global read role does, with its values at the instant. The storetest
subtests `ACLGateReadsTodaysEdges` and `ACLGateJudgesTheRowToday` pin both
cases on PostgreSQL and SQLite.

Field (`visible:`) visibility follows the history endpoint's fail-closed rules
(`affordances.WithHistoricalSubject`, TKT-73C6B2). Only roles held globally
show fields, a grant whose condition reads a relation hides its field, and a
type some role gates with `visible:` is closed by default. A property
condition is judged on the row as it stood at the instant, as history judges
it on the version. This is the one limit of failing closed: a field condition
on the row's own values (for example a classification) is not checked again
against today's row. Search compiles the same rules into its field scope.

A `related(...)` traversal is data, so it reads the edges at the instant.
The read gate folded onto its endpoint is marked
(`store.RelationPredicate.Gate`) and reads today's edges like any other
gate, in SQL on both backends and in SQLite's Go fallback
(`store.ReadToday`). The storetest subtest
`GateFoldedIntoATraversalReadsToday` pins it.

A read of the past is a history read, so it needs the `history:read`
permission, as the history endpoint does. Without it the request answers
403 `as_of_forbidden`, naming the permission.

We do not reconstruct the ACL as it stood at the instant.
