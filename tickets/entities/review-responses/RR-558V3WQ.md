---
id: RR-558V3WQ
type: review-response
title: 'Test gaps: operator colour served, collapsed tone, enum fallback'
finding: No tests for an operator colour class, a collapsed column dropping its tone, or the enum fallback on both axes.
severity: minor
resolution: TestResolveKanbanCellClasses covers custom-<name>; the tones validation case uses the enum fallback on both axes; KanbanView.cellColors.test.ts covers the collapsed column.
status: addressed
---
