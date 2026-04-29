---
id: RR-PSJY9
type: review-response
title: Inconsistent invalid/empty sentinel between formatValue and formatCellValue
finding: formatValue returns '-' for null/undefined; formatCellValue returns ''. formatDate returns '-' for invalid dates regardless of caller. So a malformed date in a list cell renders '-' while a null cell renders ''. Either reconcile (have formatDate return null and let callers substitute), pass an invalidSentinel parameter, or document the divergence in JSDoc.
severity: significant
resolution: 'formatDate now returns string | null — null for invalid input. Each caller substitutes its own sentinel: formatValue uses ''-'' (matches its null/undefined contract), formatCellValue uses '''' (matches its empty-cell contract). Updated formatCellValue''s invalid-date test from toBe(''-'') to toBe('''') and renamed it to make the cell-empty contract explicit.'
status: addressed
---
