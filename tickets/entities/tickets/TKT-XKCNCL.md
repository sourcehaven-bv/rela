---
id: TKT-XKCNCL
type: ticket
title: Push down query scopes built from related() and equalities
kind: enhancement
priority: high
effort: xl
status: done
---

## Description

Any data-entry `query_scopes:` entry turns off list pushdown
(`internal/dataentry/api_v1.go:399`). The traversal resolver's `MatchingIDs`
then receives every candidate ID of the type, not one page. The count also comes
from the full candidate set. Cost grows with the size of the type.

Fix: lower a scope that is a plain AND of `related()` terms and constant
equalities into `store.GraphQuery`. The list then keeps full pushdown: paging,
ordering and the scoped count through `store.CountMatched`.

The ACL read gate occupies `GraphQuery.HasInbound` for any principal reading a
type through a role relation. So `GraphQuery` gains `Related`, a conjunctive,
caller-only list of directed relation predicates, implemented in pgstore and
`graphquerynaive` and pinned in storetest.

Scopes with `or`, `not` (including `not related`) or other functions fall back
to the current path. The nested half of TKT-44PVX2 still applies. SQL pushdown
on sqlite is TKT-B51CYD.

Acceptance:

- A `storetest.Counting` budget test shows the same store-query count at 10
and 50 rows for a scoped list.
- A pg EXPLAIN test shows the pushed-down shape uses the derived index.
- Scopes that cannot be lowered keep today's behaviour.

Follow-up to TKT-CXQEV0 (PR #1669). Stacked on that PR.
