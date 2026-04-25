---
id: RR-8EVKJ
type: review-response
title: Composable did not auto-reset when entityType changes
finding: useEntityIDControls only resets when reset() is called explicitly (from initializeDefaults() in DynamicForm.vue, run on mount + selectTemplate). Since RouterView reuses FormView/DynamicForm across /form/:id param changes (no :key, no watch on props.formId), navigating from /form/tag to /form/module kept stale manualId and selectedPrefix visible. buildPayloadFields gates on showManualIDInput so it wouldn't leak between modes, but the stale value in the input box was visible to the user.
severity: significant
resolution: 'Added a watcher in useEntityIDControls.ts that calls reset() when the entityType identity changes, the id_type changes, or the prefix list changes. Three new test cases in useEntityIDControls.test.ts pin the behavior: clears manualId on manual→short transition, updates selectedPrefix to first of new list, does not reset when same object is reassigned.'
status: addressed
---
