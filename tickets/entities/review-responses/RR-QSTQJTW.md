---
id: RR-QSTQJTW
type: review-response
title: Reorder handle cell picked up the table cell rule
finding: 'EntityDetail .data-table :deep(td) (0,2,1) outranked .ordered-row__handle-cell (0,2,0): handle cells got 10px 12px padding.'
severity: significant
resolution: Cell rules now use :deep(td:not(.ordered-row__handle-cell)); OrderedSectionRow comment updated.
status: addressed
---
