---
id: RR-3WKYQX
type: review-response
title: ConditionPrefilterer contract omitted the raw-vs-redacted asymmetry
finding: The pre-filter compares RAW store values while Match evaluates the field-REDACTED candidate (redactedForSuggestion). Sound today because redaction removes a property (binds Nil, every current-user form is false on Nil), so the Go pass is strictly narrower — but the contract prose described the two passes as seeing the same values.
severity: nit
resolution: ConditionPrefilterer's doc names the asymmetry and why it is sound; TestNextAction_HiddenPropertyMakesCurrentUserConditionFalse pins that a visible:-hidden assignee makes is_current_user non-matching end to end while the unhidden baseline fires.
status: addressed
---
