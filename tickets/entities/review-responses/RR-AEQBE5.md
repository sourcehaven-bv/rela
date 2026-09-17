---
id: RR-AEQBE5
type: review-response
title: 'IB-review: a list condition: over a hidden field decides row membership'
finding: 'IB-review (CISO, github:rela#1593): a list `condition:` is evaluated against the raw, un-field-redacted entity.Properties, because field redaction happens later at serialization. Since `condition:` supports current_user/is_current_user/has_current_user, a condition over a property hidden from the reader by `visible:` decides whether the row appears — leaking the value through row presence/absence. The same code path elsewhere (nextaction.go, redactedForSuggestion) already applies the deliberately safe approach, which was not carried over here. Grounds: POLICY-015 §3, CONTROL-5-15.'
severity: critical
resolution: 'Confirmed by reproducing it first: with `assignee` hidden, a list whose condition is is_current_user(entity.assignee) still returned the matching row. applyViewCondition now takes a redactor and evaluates the REDACTED candidate, matching redactedForSuggestion; the kept rows remain the originals, since response redaction happens at serialization. Both call sites (list read and list export) pass it. Pinned by TestViewCondition_HiddenPropertyMakesCurrentUserConditionFalse and TestViewCondition_RedactionDoesNotWidenTheList, each verified to fail with the redaction removed. The same exposure exists in the query-scope path shipped by #1609 and is tracked separately as BUG-8NEKDP, because fixing it also requires declining the store pushdown for scopes over redactable properties.'
status: addressed
---

## Why post-redaction evaluation is sound

Redaction REMOVES a hidden property rather than blanking it, so it binds Nil,
and every current-user form is false on Nil. The Go pass is therefore strictly
narrower than an unredacted one for exactly the forms that can name the reader —
the asymmetry `ConditionPrefilterer` already documents for next actions.

## The negation case, stated explicitly

A NEGATED per-user condition (`not is_current_user(...)`) becomes true for every
row once the property binds Nil, so the list GROWS rather than shrinks. This
discloses nothing — every row is treated identically, so no row is
distinguishable from another — and is the accepted trade.
`TestViewCondition_RedactionDoesNotWidenTheList` records it so a later reader
does not mistake it for a regression.

## Scope of the fix

The finding named the list `condition:` path, which is what this PR introduces
and what is fixed here. While verifying it I found the same shape in
`applyScope` for `query_scopes:`, which is already on develop via #1609 — filed
as BUG-8NEKDP (high) rather than fixed here, since it additionally requires
declining the store pushdown for scopes touching a redactable property and
interacts with RR-U4H8BQ.

The general rule is recorded as `AM-row-membership-never-reads-a-hidden-field`,
which also draws the line against `AM-feed-field-redaction`: a predicate that
CANNOT name the reader redacts after (so membership stays
principal-independent), one that CAN redacts before.
