---
id: RR-Y3W2UPK
type: review-response
title: Guard binding is tied to the message source
finding: GuardHolds binds the message variable. Moving it into the shared engine puts a channel word inside it, and later sources need another binding.
severity: significant
resolution: 'Addressed in plan: the binding stays in realtimecfg; flow.Decide takes a guard callback per branch, so each source binds its own variables.'
status: addressed
---
