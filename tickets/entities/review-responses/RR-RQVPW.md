---
id: RR-RQVPW
type: review-response
title: InlineCreateModal does not reset state when entityType prop changes mid-open
finding: InlineCreateModal.vue:47-53 watcher on props.show resets state only when show flips true. If entityType prop changes while the modal is already open (unlikely but possible via parent prop flip), composable state stays stale.
severity: nit
resolution: RR-8EVKJ added an entityType watcher inside useEntityIDControls itself, so the auto-reset now fires regardless of which consumer holds the composable. Both DynamicForm and InlineCreateModal benefit without component-level changes.
status: addressed
---
