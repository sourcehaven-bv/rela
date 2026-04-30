---
id: RR-A6LEV
type: review-response
title: 'Dirty-flag clearing order: dirty=false before return ok could fight a downstream guard cancellation'
finding: DynamicForm.vue:567 sets dirty=false before returning ok. If a global beforeResolve later cancels, dirty was wrongly cleared. rela has no global guards so this is currently harmless. Add a one-line comment documenting the assumption.
severity: minor
resolution: Added comment in DynamicForm.vue route guard explaining the dirty=false-before-return is safe because rela has no global beforeResolve guards. If one is added, the assignment should move to a router.afterEach hook.
status: addressed
---
