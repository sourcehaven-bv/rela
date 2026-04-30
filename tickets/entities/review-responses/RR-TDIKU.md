---
id: RR-TDIKU
type: review-response
title: beforeunload coverage and internal router.push paths not asserted
finding: 'Plan does not explicitly assert that internal router.push calls (DynamicForm.vue:390/392/402 on save success) do not trigger the modal — today they don''t because dirty.value is cleared first; with the await-in-guard fix this stays fine, but the assertion belongs in the plan and the test list. Also: no nested-form scenario considered (side panel forms triggering navigation). Verify there''s no nesting today and document this.'
severity: critical
resolution: 'Plan now states the invariant: internal save-success router.push calls (DynamicForm.vue:390/392/402) clear dirty.value first, so the guard short-circuits via ''if (!dirty.value) return true''. Test added: ''guard returns true without prompting when dirty=false''. No nested-form scenario exists today (verified by grep — DynamicForm is mounted only as a route component, never inside another DynamicForm).'
status: addressed
---
