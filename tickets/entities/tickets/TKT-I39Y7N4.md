---
id: TKT-I39Y7N4
type: ticket
title: Push list conditions and non-lowering scopes into the store
kind: enhancement
priority: medium
effort: m
status: backlog
---

## Description

Follow-up of TKT-F2EJQY8. A list with a `condition:`, and a list under a query
scope that `queryplan.LowerScope` does not lower exactly, stay on the Go path.
That path reads every header of the type through `scopedHeaders`
(`internal/dataentry/scopedread.go`) and judges the condition or scope in Go, so
the rows read grow with the store.

Steps:

- Push a list `condition:` into the list's `GraphQuery` when it lowers
exactly. A condition is judged on the REDACTED row today, so push it only when
every property it reads is visible on every row to the principal (the
`gatedListQuery` rule for filters), and gate each `related(...)` with the
request's traversal gate, as scopes do (TKT-XKCNCL).
- Count with `store.CountMatched` and page in the store, as
`listpushdown.go` does for scopes.
- Widen `LowerScope` where a non-lowering scope shape is common.
- Convert the calls-only budget tests to rows+calls:
`TestViewCondition_RelatedIsSizeIndependent`,
`TestViewCondition_RelatedWithTraversingGrantIsSizeIndependent`,
`TestQueryBudget_NavItemsRedactionIsSizeIndependent` (condition case),
`TestQueryBudget_TraversalScopeIsSizeIndependent`.
