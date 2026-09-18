---
id: AM-row-membership-never-reads-a-hidden-field
type: automated-measure
title: A per-user predicate that decides row membership evaluates the REDACTED entity, never raw store values
description: "Every evaluation site that lets a current_user predicate decide whether a row appears must evaluate the field-redacted candidate. A hidden property then binds Nil and every current-user form is false on Nil, so the pass is strictly narrower than an unredacted one. Pinned by TestNextAction_HiddenPropertyMakesCurrentUserConditionFalse (next actions), TestViewCondition_HiddenPropertyMakesCurrentUserConditionFalse plus TestViewCondition_RedactionDoesNotWidenTheList (list condition:), and — once BUG-8NEKDP is fixed — the equivalent for applyScope. Each verified to fail with the redaction removed."
kind: test
location: internal/dataentry/nextaction_condition_test.go, internal/dataentry/viewcondition_redaction_test.go, internal/appbuild/queryscopevisibility.go (load-time detector), internal/dataentry/scopedread.go (applyScope, to be covered with BUG-8NEKDP)
status: active
---

Pins BUG-8NEKDP, and the IB-review finding on PR #1593.

Field redaction strips a hidden property from the RESPONSE, so its value never
reaches the wire. That is not sufficient on its own. When a per-user predicate
decides **membership**, the surviving row set answers the predicate once per
row — so a reader recovers a hidden value one bit at a time without it ever
being serialized. The remedy is to evaluate the redacted candidate, which makes
the hidden property bind Nil.

**This is a rule about PER-USER predicates, not about redaction order in
general.** `AM-feed-field-redaction` pins the OPPOSITE order for the ICS feed
on purpose: there the `where:` clause is operator-authored and principal
independent, and redacting first would make feed membership vary per principal
— the same divergence, arriving from the other direction. The distinguishing
question is whether the predicate can name the reader:

- A predicate that **cannot** name the reader (`where:`, a static filter) is the
  same for everyone. Redact AFTER, so membership stays principal-independent.
- A predicate that **can** (`current_user`, `is_current_user`,
  `has_current_user`) is already per-principal. Redact BEFORE, or the row set
  becomes a per-row oracle on whatever field it names.

A new evaluation site must state which case it is and carry the matching test.

**The query-scope case already has a load-time detector.**
`appbuild.QueryScopeVisibilityConflicts` compares every compiled scope's
`entity.*` attributes against the properties any role can hide with `visible:`,
and warns per (scope, property). It treats `visible:` as a closed world (the
restricted set is the COMPLEMENT of what a role grants, so a property added
later is restricted automatically) and counts a conditional `when:` grant as
NOT granting. That is the operator-facing half of this measure, and a fix for
BUG-8NEKDP should keep it rather than invent a second signal.

The detector covers `query_scopes:` ONLY — it is driven by
`scopes.Compiled.Each`, and a view `condition:` is compiled elsewhere
(`internal/conditionlint`). So the surface this PR adds has the runtime
protection and not the load-time warning, the mirror image of the scope path,
which has the warning and not the protection. Extending the check to compiled
view conditions is a small, obvious follow-up; both halves are worth having,
since the runtime fix silently changes a row set while the warning tells the
operator which declaration to reconsider.

It warns rather than refuses, on the argument that the ACL read gate runs first
so every affected row is one the reader may already fetch. That is a fair bound
on SEVERITY and not a reason to evaluate raw values: the reader is entitled to
the row and not to the value. The detector names the contradiction; this
measure decides which side wins at runtime.

The same split decides what may be PUSHED to the store. A per-user conjunct
must not become a SQL predicate over a redactable column, because the database
evaluates it below any redaction; a principal-independent one may. Today
`QueryScopeResolver.Resolve` gets this right by accident of a different goal —
it passes an empty identity to `ConditionPrefilters`, so `current_user`
conjuncts never lower — which is why fixing BUG-8NEKDP needs no pushdown
change. If that call ever gains a real identity (RR-U4H8BQ wants the index it
costs), the redaction rule has to move with it.

Note that redacting first makes a NEGATED per-user condition match every row,
because Nil is uniform. The list grows rather than shrinks. That discloses
nothing — every row is treated identically — and is the accepted trade;
`TestViewCondition_RedactionDoesNotWidenTheList` records it so it is not
mistaken for a regression later.
