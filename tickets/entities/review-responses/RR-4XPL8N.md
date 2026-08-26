---
id: RR-4XPL8N
type: review-response
title: Duplicated storeRelationLookup risks one-directional drift
severity: minor
status: deferred
finding: >-
  storeRelationLookup now exists twice — internal/dataentry/affordances_policy.go
  and internal/appbuild/relationlookup.go — because appbuild cannot import
  dataentry. Both feed affordance `when:` predicates (has_relation,
  count_relations), so a bug fixed in one and missed in the other makes two
  security surfaces disagree about who may see what. Both also swallow store
  iteration errors and return "no edge", which is fail-OPEN for a
  `when: not has_relation(...)` predicate.
reason: >-
  Deferred to TKT-0XL8MF (the write-gate ticket), which already reshapes this
  exact wiring — it must reach the *affordances.PolicyResolver that
  buildFieldRedactor currently wraps and discards, so it touches both copies
  anyway. Consolidating here would mean a second pass over the same seam within
  one ticket-cycle. Mitigated meanwhile: each copy cross-references the other,
  and the debt (including the fail-open error-swallow) is recorded on
  TKT-0XL8MF under "Known debt this ticket can retire". Not open-ended — it is
  a named item on a filed ticket that depends on this one.
resolution: >-
  Deferred to TKT-0XL8MF, which already touches this wiring and is the natural
  place to hoist the adapter into internal/affordances and delete both copies.
  Mitigated meanwhile with a cross-reference comment on each copy so a fix to
  one is not silently missed in the other. Recorded on that ticket under
  "Known debt this ticket can retire", including the fail-open note.
---

Duplication was the right call over the alternatives: `appbuild -> dataentry`
would be a wiring-package cycle, and dataentry's copy is coupled to the
`policyResolver` beside it. The consolidation belongs in a change that is
already reshaping this seam, not bolted onto a redaction fix.
