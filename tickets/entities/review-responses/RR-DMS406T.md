---
id: RR-DMS406T
type: review-response
title: Places that show a world name were missed
finding: DynamicForm.vue:350 passes the raw worldParam as {world}; HistoryView.vue:351 shows the raw world name in history.noFace.
severity: significant
resolution: 'Addressed in plan: both use schemaStore.worldLabel.'
status: addressed
---
