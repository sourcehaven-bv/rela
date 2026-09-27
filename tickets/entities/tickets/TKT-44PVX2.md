---
id: TKT-44PVX2
type: ticket
title: 'related(): chained incoming hop into a relation-granted type'
kind: enhancement
priority: low
status: backlog
---

## Description

A query scope such as `related(entity, {'implementedBy', 'reportedBy'})` fails
with 422 `query_scope_unsupported` for a principal who reads the intermediate
type through a role relation (for example `editor-of`). The ACL read gate and
the next incoming hop both need the endpoint's single `HasInbound` slot.

Fix: let `store.EndpointPredicate` carry a conjunction of inbound predicates,
implemented in pgstore, sqlitestore and graphquerynaive, and pinned in
storetest. Then `lowerTraversal` can place both.

Found in the TKT-CXQEV0 review (RR-3RIDCH).
