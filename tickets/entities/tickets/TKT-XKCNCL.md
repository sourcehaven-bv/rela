---
id: TKT-XKCNCL
type: ticket
title: Push down query scopes built from related() and equalities
kind: enhancement
priority: high
effort: l
status: ready
---

## Description

Any data-entry `query_scopes:` entry turns off list pushdown
(`internal/dataentry/api_v1.go:399`). The traversal resolver's `MatchingIDs`
then receives every candidate ID of the type, not one page. The count also comes
from the full candidate set. Cost grows with the size of the type.

Fix: lower a scope that is a plain AND of `related()` terms and equalities into
`store.GraphQuery` (`HasInbound`/`HasOutbound` plus property equalities). The
list then keeps full pushdown: paging, ordering and the scoped count through
`store.CountMatched`.

Limits:

- `GraphQuery` has one inbound slot and one outbound slot. A scope that needs
more, or that competes with the ACL gate for the same slot, falls back to the
current path (see TKT-44PVX2).
- `not related(...)` needs a NOT EXISTS form that `GraphQuery` lacks. It falls
back too.

Acceptance:

- A `storetest.Counting` budget test shows the same store-query count at 10
and 50 rows for a scoped list.
- A pg EXPLAIN test shows the pushed-down shape uses the derived index.
- Scopes that cannot be lowered keep today's behaviour.

Follow-up to TKT-CXQEV0 (PR #1669). Stacked on that PR.
