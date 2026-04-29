---
id: RR-70BG2
type: review-response
title: Tests assert parity against normalized input rather than original input
finding: 'The RRULE: prefix and DTSTART:\n cases at format.test.ts:108-118 assert parity with `formatValue(''FREQ=DAILY'', ''rrule'')` — i.e. the normalized input, not the original. If formatValue ever changes how it normalizes, the tests still pass even when formatCellValue diverges. The actual parity contract is `formatCellValue(input, ...) === formatValue(input, ''rrule'')`.'
severity: significant
resolution: 'Refactored the rrule tests into an `it.each` table that asserts the parity contract directly: `formatCellValue(input, ''schedule'', mockEntityType)` === `formatValue(input, ''rrule'')` for the same input (bare, RRULE: prefix, DTSTART:\nRRULE:, malformed). The tests no longer pin to the normalized form, so any future change in normalization keeps the parity contract honest.'
status: addressed
---
