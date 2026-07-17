---
id: RR-F30CZ
type: review-response
title: 'Transition enforcer minor gaps: RuleID drops permission, create-guard asymmetry, empty/list-property edge cases'
finding: 'Four nits from code review: (N1) mapTransitionError sets RuleID=''-'' discarding the permission name (recoverable from Reason but not queryable by rule_id); put the permission in RuleID. (N2) create-guard asymmetry (EnforceCreate can''t emit ErrGuardDenied today) is undocumented at manager.go:436; add a comment so a future guarded-create routes through mapTransitionError. (N3) clearing a machine prop (X->'''') surfaces as ErrIllegalTransition with no test pinning it; add a table case + whitespace note. (N4) transitions on a list:true property read via GetString coincidentally ''work''; reject transitions on list properties at compile time (pd.List is available at compile.go:52).'
severity: minor
resolution: 'N1: guard denial now returns a typed *statemachine.GuardError carrying Permission; mapTransitionError puts it in Decision.RuleID (queryable). N2: comment added at CreateEntity noting the create-guard asymmetry. N3: TestEnforceUpdate_ClearAndWhitespace pins X->'''' and trailing-space as ErrIllegalTransition. N4: transitions on a list:true property now rejected at compile time (TestCompile_RejectsTransitionsOnListProperty).'
status: addressed
---

## Findings (bundled minors)

- **N1 — RuleID loses the permission name.** `mapTransitionError` sets
`RuleID: "-"` (manager.go:290). The permission is recoverable from `Reason` but
not queryable by `rule_id`. Put the permission name in `RuleID`.
- **N2 — create-guard asymmetry undocumented.** `EnforceCreate` can't emit
`ErrGuardDenied` today, so only the update path wraps it. Add a comment at the
create call site (manager.go:436) so a future guarded-create routes through
`mapTransitionError` (else it 422s instead of 403).
- **N3 — clearing a machine property.** `X → ""` on update → `ErrIllegalTransition`
(422), which is defensible but untested; whitespace values (`"approved "`) pass
raw string compare. Add a table case documenting empty/whitespace semantics.
- **N4 — list-typed machine property.** Nothing rejects `transitions` on a type
used by a `list: true` property; `GetString` coincidentally returns a scalar.
Reject at compile time — `pd.List` is available in the indexing loop
(compile.go:52).
