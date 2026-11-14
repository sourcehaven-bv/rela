---
id: TKT-I39Y7N4
type: ticket
title: Push list conditions and non-lowering scopes into the store
kind: enhancement
priority: medium
effort: m
started: "2026-11-14"
completed: "2026-11-14"
status: done
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

## Outcome

- `queryplan.LowerCondition` is a separate, wider lowering for query
conditions: AND of `==`, `~=`, memberships (`has_current_user`, `'x' in
entity.list`), `current_user.id`, `related(...)` and `not related(...)`.
`queryplan.LowerScope` is unchanged and stays the lowering for `visible:`.
TKT-I3MGCRT (desktop notifications) can reuse `LowerCondition` instead of
`LowerScope` plus `ConditionPrefilters`.
- A list `condition:` is pushed into the list's store query when it lowers
and every field it reads is visible on every row to the principal. The list API,
nav items and dashboard cards (which page through `listPage`) are then counted
with `CountMatched` and paged in the store.
- Nav status rules that lower are one scoped count each.
- List query scopes go through `LowerCondition`, so scopes using `~=`,
memberships or `not related(...)` are pushed too.
- `store.PropPredicate.Scalar` now also applies to `PropNotEqualOrEmpty` on
every backend, so a scalar `~=` matches Go on off-type stored values.

## Behaviour changes

None in the rows or totals a client sees: the differential tests compare the
store path with the Go path per principal on memstore, sqlite and postgres.
Faster responses for the shapes above. Nav status counts are now separate counts
per rule, so a write between two counts can change which rule wins for that one
request.

## Left out

Ordered comparisons (TKT-08JN7C3), kanban and export conditions (TKT-X7ECS9Z). A
list filter divergence on off-type values found on the way is BUG-42F3SYF.
