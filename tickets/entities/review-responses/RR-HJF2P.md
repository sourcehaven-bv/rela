---
id: RR-HJF2P
type: review-response
title: Array short-circuit at format.ts:55-57 swallows array-shaped rrule values
finding: The early `if (Array.isArray(value)) return value.join(', ')` at format.ts line 55-57 runs BEFORE the property-typed branch. If a backend ever serializes an rrule property as an array (multi-rule, or a single-element list), the cell will render raw iCal joined by commas while the detail page would still try to format it. That's exactly the drift the fix is supposed to prevent.
severity: significant
resolution: 'Reordered formatCellValue: typed-property branch (date/boolean/rrule) now runs BEFORE the generic Array short-circuit. The rrule branch also unwraps a single-element array via `Array.isArray(value) ? value[0] : value` before delegating to formatValue, so an rrule serialized as a one-element list still renders as human-readable text. Added regression test ''formats rrule wrapped in a single-element array'' in format.test.ts.'
status: addressed
---
