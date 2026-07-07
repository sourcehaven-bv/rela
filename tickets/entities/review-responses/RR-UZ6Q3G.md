---
id: RR-UZ6Q3G
type: review-response
title: Kanban column headers and filter/edit dropdowns show raw values
finding: 'Kanban column headers (KanbanView.vue) render the raw enum value outside any Badge, and filter dropdowns (FilterBar, AdHocFilterMenu, TagSelect options) plus the edit `<option>` list present raw values. Result: card badge says ''In Progress'' while the column header / filter option says ''in_progress''. Plan must decide per-surface: resolve the enum label for the kanban column header, the edit `<option>` text, and filter option lists (or explicitly scope-out with the inconsistency accepted in writing).'
severity: significant
status: open
---
