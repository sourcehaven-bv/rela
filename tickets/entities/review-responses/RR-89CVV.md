---
id: RR-89CVV
type: review-response
title: String(value) is redundant inside rrule branch in formatValue
finding: format.ts:28 — the `typeof value === 'string'` guard at line 25 already narrows the type, so `String(value)` on line 28 is dead defensive code.
severity: nit
resolution: Removed the redundant `String(value)` calls inside the rrule branch of formatValue. The `typeof value === 'string'` guard already narrows the type.
status: addressed
---
