---
id: TKT-JRP3ZRX
type: ticket
title: 'Atomic batch engine: entitymanager.CreateBatch'
kind: enhancement
priority: medium
effort: m
started: "2026-11-04"
completed: "2026-11-04"
status: done
---

## Description

Add `entitymanager.CreateBatch`, a package function that creates several
entities and the relations between them in one `store.Store.Tx`. It generalizes
`CreateLinked` from one entity and one edge to N entities and M edges
(RES-QKREXZZ option 1, DEC-ODU65KR).

- Every check that needs no transaction runs first, as the caller: per
entity the create checks of `prepareCreate` (face, create ACL, field gate, file
and external-ref refusals, custom id); per relation the relation ACL, the
endpoint read gate and the type tuple through `prepareRelationFrom`. A
batch-local endpoint is passed as the planned family and never looked up in the
store. The first refusal stops the batch before anything is written.
- One `Tx` with the manager deps rebound to the view writes every entity
(`createCore`, generated ids minted under the lock) and then every edge
(`writeRelationCreate`, capacity and owning rules on the view).
- After commit: audit records and alias notifications for every row, then
on-create automations and cascades per entity as follow-ups. A follow-up failure
is reported per operation and does not undo the commit.
- On a store that cannot roll back, rows a failed transaction left behind
are audited, as `CreateLinked` does with `finishKept`.
