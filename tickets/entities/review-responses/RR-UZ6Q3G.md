---
id: RR-UZ6Q3G
type: review-response
title: Kanban column headers and filter/edit dropdowns show raw values
finding: 'Kanban column headers (KanbanView.vue) render the raw enum value outside any Badge, and filter dropdowns (FilterBar, AdHocFilterMenu, TagSelect options) plus the edit `<option>` list present raw values. Result: card badge says ''In Progress'' while the column header / filter option says ''in_progress''. Plan must decide per-surface: resolve the enum label for the kanban column header, the edit `<option>` text, and filter option lists (or explicitly scope-out with the inconsistency accepted in writing).'
severity: significant
resolution: Kanban column headers use a new columnTitle() that resolves the enum label (explicit config label still wins). Edit-mode <option> text in SelectWidget shows the label; MultiSelect passes optionLabels to TagSelect. Filter dropdowns (FilterBar optionText, AdHocFilterMenu valueLabel) show the label while the filter value stays raw. Verified by KanbanView.test.ts and AdHocFilterMenu.test.ts.
status: addressed
---
