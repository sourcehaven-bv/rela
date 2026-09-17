---
id: AM-row-membership-never-reads-a-hidden-field
type: automated-measure
title: A per-user predicate that decides row membership evaluates the REDACTED entity, never raw store values
description: "Every evaluation site that lets a current_user predicate decide whether a row appears must evaluate the field-redacted candidate. A hidden property then binds Nil and every current-user form is false on Nil, so the pass is strictly narrower than an unredacted one. Pinned by TestNextAction_HiddenPropertyMakesCurrentUserConditionFalse (next actions), TestViewCondition_HiddenPropertyMakesCurrentUserConditionFalse plus TestViewCondition_RedactionDoesNotWidenTheList (list condition:), and — once BUG-8NEKDP is fixed — the equivalent for applyScope. Each verified to fail with the redaction removed."
kind: test
location: internal/dataentry/nextaction_condition_test.go, internal/dataentry/viewcondition_redaction_test.go, internal/dataentry/scopedread.go (applyScope, to be covered with BUG-8NEKDP)
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

Note that redacting first makes a NEGATED per-user condition match every row,
because Nil is uniform. The list grows rather than shrinks. That discloses
nothing — every row is treated identically — and is the accepted trade;
`TestViewCondition_RedactionDoesNotWidenTheList` records it so it is not
mistaken for a regression later.
