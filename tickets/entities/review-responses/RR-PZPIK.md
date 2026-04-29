---
id: RR-PZPIK
type: review-response
title: Null/empty-string asymmetry between formatValue and formatCellValue is load-bearing but undocumented
finding: formatValue(null, 'rrule') returns '-'; formatCellValue(null, 'schedule', ...) returns ''. The delegation makes this divergence subtler. If anyone 'fixes' formatCellValue to delegate null/undefined to formatValue, they'll silently change every existing cell rendering. The behaviour is intentional but undocumented in the code.
severity: significant
resolution: Added a comment at the top of formatCellValue explaining that null/undefined deliberately returns '' (not '-' as formatValue does) so blank table cells stay visually quiet, and warning future maintainers not to delegate that branch to formatValue.
status: addressed
---
