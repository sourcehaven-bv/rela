---
id: RR-JFCG7V6
type: review-response
title: Missing tests for step change and label-less hint
finding: No test moved between two steps with details, or checked the generic button name without a label.
severity: minor
resolution: DynamicForm.help.test.ts moves to a second step and checks its details are closed (fails without the :key); FieldShell.test.ts checks the generic name.
status: addressed
---
