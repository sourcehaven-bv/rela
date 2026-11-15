---
id: TKT-08JN7C3
type: ticket
title: Lower ordered comparisons in list conditions and scopes
kind: enhancement
priority: low
effort: s
status: backlog
---

## Description

Follow-up of TKT-I39Y7N4. `queryplan.LowerCondition` refuses ordered comparisons
(`<`, `<=`, `>`, `>=`), because in Go an ordered comparison on an unset value is
an evaluation error (`ErrNilOperand`) that fails the request, while a store
range predicate just drops the row. A list or scope using one stays on the Go
path and reads every header of the type.

Lower an ordered comparison only where Go cannot hit nil: the property is
required, or the comparison is guarded by a set-check the lowering can see.
Dates need the typed comparison the store uses for `PropGreaterEqual`. Extend
`TestListPushdown_ConditionMatchesGoPath` with the new shapes.
