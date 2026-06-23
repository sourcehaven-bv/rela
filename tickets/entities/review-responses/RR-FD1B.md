---
id: RR-FD1B
type: review-response
title: 'Round 1 #2: key-set invariant between _props and _fields'
finding: |
  Plan handwaves "consistency" between _props and _fields. The actual invariant is: hidden ⇒ absent from both; `writable: false` ⇒ may be present in _fields only (as a deny verdict); default-writable + value-present ⇒ present in both. Without pinning this rule a future consumer can drift, e.g. computeFieldAffordances changes its hidden-stripping behaviour and the two maps disagree.
severity: significant
status: addressed
resolution: |
  PLAN AC 8 amended: add an explicit test asserting `keys(_props) ⊆ keys(e.Properties) \ hidden(e)` AND `keys(_fields) ∩ hidden(e) == ∅`. The two maps may legitimately diverge in one direction only: a property may appear in _fields (e.g. `writable: false` verdict) without appearing in _props if the entity has no stored value for it. The reverse (property in _props but hidden in _fields) is a contract violation.

  Documented in the V1ViewEntity Go doc comment so future maintainers see the rule at the source.
---
