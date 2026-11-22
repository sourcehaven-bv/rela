---
id: RR-KEHU9OU
type: review-response
title: JSON.stringify change detection depends on key order
finding: FlowStepForm.vue compares values with JSON.stringify, so an untouched object value with reordered keys could be sent as a change. Use a deep-equal helper.
severity: nit
resolution: Moved deepEqual from useAutoSave into src/utils/deepEqual.ts (with tests) and used it in FlowStepForm, so key order no longer matters.
status: addressed
---
