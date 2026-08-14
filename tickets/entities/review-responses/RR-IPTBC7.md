---
id: RR-IPTBC7
type: review-response
title: Empty list-valued cells rendered an em-dash placeholder
finding: 'MultiSelectWidget renders an em-dash for an empty array (RR-UD2C) so ''no value'' reads as distinct from a loading state. That is a DETAIL-view contract. formatCellValue documents the opposite for cells: ''Cells render empty for null/undefined (vs - in formatValue) so blank table cells stay visually quiet; do not delegate this branch to formatValue.'' A sparsely-populated list column became a column of dashes. It also interacted with the mobile predicate: desktop dashed while mobile dropped the column, so the same data rendered two ways by viewport.'
severity: critical
resolution: Added isDenseEmpty(value) to widgets/viewRouting.ts, shared by both surfaces. The CELL decides emptiness before the widget sees the value, so a widget placeholder can never reach a dense surface. MultiSelectWidget is unchanged - its em-dash is correct for detail views and other consumers depend on it.
status: addressed
---
