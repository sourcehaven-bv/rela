---
id: TKT-205V2N
type: ticket
title: 'related() in views, next-action, CLI filter, validation, automation, state machine and ACL when:'
kind: enhancement
priority: medium
effort: xl
status: review
---

## Description

`related()` works only in data-entry `query_scopes:` (TKT-CXQEV0). The CLI
`--filter`, automation, validation, the state machine and ACL `when:` accept it
at load and then fail each time it runs:

```text
related: no traversal resolver is bound; this program must be lowered into a
store query or evaluated with Bindings.SetTraversal
```

Work:

1. Move `answerTraversals` (`internal/appbuild/queryscopetraversal.go`) into
one shared resolver. Input: a program, an entity type, candidate IDs, a read
gate and a store query (`MatchingIDs`). One entity is a list of one.
2. Wire the resolver into view `condition:`, next-action `condition:` (kept out
of the SQL prefilter), CLI `--filter` (allow-all gate, operator trust),
validation when/then (batch, `deps.VisibleReader`) and automation `on.condition`
(pass a store in). Each surface gets a test that pins its store-query count.
Document that an automation check at trigger time is not re-run when the related
entity changes.
3. Bind the traversal into the state-machine `When:` env on both the write
side (`EnforceUpdate`) and the read side (`Performable`), both reading the raw
store: a hidden blocker must still block, and a gated read side would only move
the one-bit channel to the 422. Moving `When:` onto `predicatefns.Evaluator` is
TKT-BZBN2O.
4. Support `related()` in ACL `when:` (field, visible, option and relation
grants). Bind the shared resolver into the affordance bindings
(`internal/affordances/bindings.go:96`); list redaction evaluates a page of rows
as one batch. The traversal reads the raw store, since the gate cannot gate
itself. A `when:` may therefore depend on a property of an entity the principal
cannot see, and reveal one bit about it through whether a field is shown or
writable. The operator authors that policy; document it in
`docs/acl-security.md`.
5. Refuse `related()` at load for form conditions, which run in the browser.

Follow-up to TKT-CXQEV0 (PR #1669).

## Deviations from the plan

- No separate boot check for unscoped validation rules. A rule using
`related()` reports a load error when it runs and no binder is wired.
- View and next-action conditions are not refused at load when a traversal
compares a conditionally visible property: `ViewConditions` has no ACL policy.
`Request.GateTraversal` refuses it per request instead. ACL grants get a startup
warning (`warnConditionallyVisible`).
- `ErrConditionUnavailable` was not needed; existing errors cover every
refusal.
- Derived traversal indexes cover view and next-action conditions. The query
shape is the one the existing EXPLAIN tests pin on both backends, so no new
EXPLAIN test was added.
