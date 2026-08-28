---
id: RR-YWIN6T
type: review-response
title: useConfirm is a singleton whose in-flight promise is shared with the navigation guard
finding: useConfirm.ts:118 returns the pending promise to a concurrent caller. DynamicForm's onBeforeRouteLeave already calls confirm() on a failed commit. With a clear-confirm dialog open, hitting Back makes the navigation guard receive the answer to the clear question - one click answers both. DynamicForm.vue:1434 already documents this hazard for the embedded case; the plan adds a second, far more frequent caller without noticing.
severity: critical
resolution: 'Approach step 5: the navigation guard must refuse to run while a proposal is undecided; if that proves awkward the clear-confirm gets its own dialog primitive instead of the singleton. useConfirm.ts added to Files to Modify; edge case added.'
status: addressed
---
