---
id: RR-KFW02L1
type: review-response
title: Transition error leaks a hidden value
finding: The elevated update skips the field gate, so a caller can write a property hidden by visible:. A refused transition quoted the stored value, and the action endpoint returned that text.
severity: significant
resolution: ScopedUpdates.PatchEntity replaces state-machine errors and the transition-guard 403 with value-free messages that keep their status class. TestScopedUpdates_Transitions checks that no value is named.
status: addressed
---
