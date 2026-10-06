---
id: RR-U47GEY
type: review-response
title: Form does not reload when the world changes
finding: The world switcher stays visible on the form route. A world change only edits the query, so the form keeps the old world's relations while the picker searches the new one.
severity: minor
resolution: DynamicForm watches formWorld in edit mode, commits pending autosave, then reloads the entity. Covered by a second e2e test that switches world in an open form.
status: addressed
---
