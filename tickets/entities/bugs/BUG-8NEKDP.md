---
id: BUG-8NEKDP
type: bug
title: A query scope over a visible:-hidden property decides row membership, leaking the value through row presence
description: 'applyScope (internal/dataentry/scopedread.go) evaluates a query scope against the RAW store header properties, before field redaction. A scope naming is_current_user(entity.<field>) over a field the reader cannot see therefore decides whether each row appears, leaking the hidden value one bit at a time through row presence/absence without it ever being serialized. Same class as the IB-review finding that blocked PR #1593 for the list `condition:` path, and the same remedy the next-action path already applies (evaluate the redacted candidate; a hidden property binds Nil and every current-user form is false on Nil). Shipped on develop via #1609 (TKT-EVR2TU).'
priority: high
status: backlog
---

## Reproduction

Confirmed on `develop` at 5dcc1c7f, using the existing scope fixture:

```go
app := newScopeTestApp(t)   // `mijn` = is_current_user(entity.toegewezen_aan)
app.fieldResolver = fakeResolver{fv: FieldVerdicts{
    Visible: map[string]bool{"toegewezen_aan": false}}}
rec := scopeListAs(app, "alice", "query_scope=mijn")
// status=200 ids=[TAAK-1 TAAK-3]  ← alice's rows, selected by a field she cannot see
```

The reader may legitimately READ those tickets; what they may not see is the
assignee. Membership still tracks it exactly, so the list answers "is
`toegewezen_aan == alice`?" per row.

## Why it matters even though no value is serialized

Field redaction strips the property from the response, so the value never
appears on the wire. The channel is the ROW SET: a reader who can vary the scope
(or simply compare two scopes) reads the predicate's answer per row. For an
identity scope that is precisely the hidden value, recovered one row at a time.

## Remedy

`internal/dataentry/nextaction.go` already documents and implements the safe
shape (`redactedForSuggestion`, pinned by
`TestNextAction_HiddenPropertyMakesCurrentUserConditionFalse`): evaluate the
REDACTED candidate. Redaction removes a hidden property, so it binds Nil, and
every current-user form is false on Nil — the pass becomes strictly narrower
than an unredacted one, never wider.

The list `condition:` path was fixed the same way in PR #1593
(`applyViewCondition` now takes a redactor). `applyScope` needs the equivalent.

Two wrinkles specific to scopes, which is why this is its own ticket rather than
a line in that PR:

1. **Store pushdown.** `ScopeProps` lowers store-safe conjuncts into the
`GraphQuery`, so part of the scope is evaluated by the DATABASE against raw
columns, below any redaction. Fixing only the Go pass leaves the pushed
conjuncts deciding membership on raw values. The pushdown for a scope touching a
redactable property likely has to be declined, which interacts with RR-U4H8BQ
(identity conjuncts deriving a dead index).
2. **Headers, not entities.** `applyScope` works on `store.EntityHeader`, while
the redaction helpers take `*entity.Entity`. The seam needs a header-shaped
redactor or a different placement.

## Negation widens, and that is accepted

Post-redaction a hidden property binds Nil uniformly, so `not
is_current_user(...)` becomes true for every row and the list GROWS. That
reveals nothing (all rows are treated identically) and is the same trade the
condition path took; worth stating explicitly so it is not mistaken for a
regression.
