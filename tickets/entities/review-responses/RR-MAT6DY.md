---
id: RR-MAT6DY
type: review-response
title: Keep entity API work out of MilkdownEditor
finding: MilkdownEditor.vue is 827 lines and would gain API orchestration.
severity: minor
resolution: 'Plan: editor emits {pos, text} and exposes applyTaskItemResult; a useTaskItemConversions composable and a menu component do the API work for DynamicForm and EntityBody.'
status: addressed
---
