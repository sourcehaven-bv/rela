---
id: BUG-8NEKDP
type: bug
title: A query scope over a visible:-hidden property decides row membership, leaking the value through row presence
description: 'applyScope (internal/dataentry/scopedread.go) evaluates a query scope against the RAW store header properties, before field redaction. A scope naming is_current_user(entity.<field>) over a field the reader cannot see therefore decides whether each row appears, leaking the hidden value one bit at a time through row presence/absence without it ever being serialized. Same class as the IB-review finding that blocked PR #1593 for the list `condition:` path, and the same remedy the next-action path already applies (evaluate the redacted candidate; a hidden property binds Nil and every current-user form is false on Nil). Shipped on develop via #1609 (TKT-EVR2TU). Scoped to IDENTITY scopes: an identity conjunct is never pushed to the store (ConditionPrefilters is called with an empty identity), so the Go-side pass is the only decider and the fix does not need to touch the pushdown. Severity is medium rather than high because appbuild.QueryScopeVisibilityConflicts already DETECTS this configuration at load and warns, and because the ACL read gate runs first so every affected row is one the reader may already fetch.'
priority: medium
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

## It is already detected at load, and that bounds the severity

`appbuild.QueryScopeVisibilityConflicts`
(`internal/appbuild/queryscopevisibility.go`) walks every compiled scope at
load time and warns when the scope reads a property any role can hide with
`visible:`. The fixture above would log:

```text
query scope "mijn" reads "toegewezen_aan", which role "..." can hide with
`visible:` — the scope shapes a row list out of a value that role may not read
```

Its doc comment argues, deliberately, for warning rather than refusing: the ACL
read gate runs FIRST and independently, so every row a scope can act on is one
the principal was already entitled to read, which makes the overlap "a
configuration smell, not a confidentiality boundary being crossed."

That argument is right about severity and wrong as a reason to leave the
evaluation raw. "The reader may already fetch the row" is the premise under
which `visible:` is still considered worth having — if it settled the question,
field-level redaction would not exist. The reader is entitled to the row and
not to the VALUE, and the scope hands over one bit of the value per row.

So: the warning names the contradiction, and this ticket decides which side
wins when an operator ships it anyway. The fix is cheap and strictly
narrowing, so losing safe is the right default. Two consequences for planning:

- Severity is **medium**, not high. Detected at load, argued about in-tree, and
  confined to rows the reader may already read.
- The fix needs no new detector. Keep the warning as the operator-facing
  signal and make the runtime behaviour match what it already claims is wrong.

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

One wrinkle specific to scopes, which is why this is its own ticket rather
than a line in that PR: `applyScope` works on `store.EntityHeader`, while the
redaction helpers take `*entity.Entity`. The seam needs a header-shaped
redactor or a different placement.

## The pushdown does NOT carry the identity conjunct

An earlier draft of this ticket claimed `ScopeProps` pushes part of an identity
scope into SQL, so the database would decide membership on raw columns below
any redaction. That is wrong, and the distinction decides how much work the fix
is.

`QueryScopeResolver.Resolve` calls
`queryplan.ConditionPrefilters(prog, meta, types, "")` with an EMPTY identity,
which makes it skip every `current_user` conjunct. Verified against the live
resolver:

```
mijn    (is_current_user(entity.toegewezen_aan)) -> Props: nil
archief (entity.status == 'gearchiveerd')        -> Props: [status == gearchiveerd]
```

So an identity scope pushes NOTHING and `applyScope` is the only thing deciding
it. Redacting the Go-side candidate is therefore sufficient for the leaking
case, and the pushdown needs no change. The empty identity is deliberate and
documented at that call site as load-bearing precisely so the pushdown stays a
strict superset. RR-U4H8BQ is about earning back the index that skipping costs;
it is adjacent, not a prerequisite.

## The literal case is a different question, and is probably fine

A scope over a LITERAL (`archief` = `entity.status == 'gearchiveerd'`) does push
to the store, and with `status` hidden it still returns exactly the archived
rows. That is not the same disclosure:

- The predicate is **operator config**, not reader input. A reader who can send
  `?query_scope=archief` can read the scope's definition in `schema.yaml` — the
  CLAUDE.md rule says config names and contents are not secret.
- It is **constant across readers**, so the row set answers "which rows are
  archived", the same question for everyone. That is the `where:`-clause shape
  `AM-feed-field-redaction` deliberately keeps un-redacted so membership stays
  principal-independent.

A per-user predicate is different because the question it answers is
*about the reader*, and each row's answer is the hidden value for that row.

This is the line `AM-row-membership-never-reads-a-hidden-field` draws. Applying
redaction to literal scopes too would be the conservative choice, but it costs
the pushdown (a redacted property cannot be a SQL predicate) and buys no
confidentiality the config does not already give away. **Decide this
explicitly before implementing** rather than redacting everything by reflex.

## Negation widens, and that is accepted

Post-redaction a hidden property binds Nil uniformly, so `not
is_current_user(...)` becomes true for every row and the list GROWS. That
reveals nothing (all rows are treated identically) and is the same trade the
condition path took; worth stating explicitly so it is not mistaken for a
regression.
