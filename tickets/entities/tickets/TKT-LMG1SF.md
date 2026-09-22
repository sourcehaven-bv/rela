---
id: TKT-LMG1SF
type: ticket
title: Cardinality check does one CountRelations per (subject, relation, direction); batch it
kind: enhancement
priority: medium
effort: m
status: backlog
---

Surfaced by TKT-CICJSN's code review; inherited from TKT-RNBLAC, not introduced
by either.

`schema.CheckCardinality` counts per subject: measured at 5 relations over 100
entities it issues 10 `ListEntities` scans and **1,000 `CountRelations` calls**.
On fsstore that is per-row file I/O; on postgres it is 1,000 round trips.

This is exactly the per-row-lookup defect CLAUDE.md's collection-reads rule
exists to prevent ("a page loads its edges with ONE `RelationQuery.EntityIDs`
query"). The inner loop should become one batched query per (relation,
direction) using `store.RelationQuery.EntityIDs`.

TKT-CICJSN is what makes this worth fixing now: it put the check behind the MCP
`analyze_cardinality` tool on the networked wiring, where an agent can invoke it
at will with no pagination and no budget.

Two related wins in the same code:

- **Scope filtering runs AFTER the full type scan** (`checkCardinalityFor`), so
`--scope` buys nothing on the read side. `store.EntityQuery` has an `IDs` field;
passing `IDs: keys(scope)` turns the scan from O(all entities of type) into
O(scope).
- Per CLAUDE.md, a new read path pins its cost with a `storetest.Counting`
budget test asserting the count is the same at 10 and 50 rows. Cardinality has
none.
