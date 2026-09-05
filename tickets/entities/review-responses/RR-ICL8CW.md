---
id: RR-ICL8CW
type: review-response
title: saveGeneration is dead code the plan makes load-bearing; RelationCards never renders in create mode
finding: saveGeneration (DynamicForm.vue:227) is declared and bound but never incremented anywhere — the plan cites it as an 'existing lever' when it is untested. Separately, RelationCards renders only when entityId is set (FormFieldList.vue:80), so it never appears on a create form; AC-4's 'RelationCards selection visually cleared' is untestable and pendingCardChanges is always empty on the create path.
severity: critical
status: addressed
resolution: >-
  Plan step 1a now states saveGeneration is dead code that this ticket makes load-bearing, and requires the bump (it is the incoming-picker fix). AC-4's RelationCards assertion removed — the widget under test is RelationPicker; pendingCardChanges noted as a create-path no-op.
---
