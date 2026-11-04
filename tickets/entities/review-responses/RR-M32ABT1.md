---
id: RR-M32ABT1
type: review-response
title: Settings defaults cast decimals to numbers
finding: SettingsView binds v-model on input type=number for defaults; a 12.50 default is saved as the number 12.5.
severity: minor
reason: Deferred to a follow-up decimal ticket; the write path now stores the number as text (12.5), so no float reaches storage; only the trailing zero of a typed default is lost.
status: deferred
---
