---
id: RR-SAIFO
type: review-response
title: DynamicForm error-handling drops non-array errors field silently
finding: 'DynamicForm.vue:412 narrowed the errors-array path with Array.isArray, falling through to a generic ''Failed to save entity'' toast. If any server path emits {errors: {field: ''msg''}} (object, not array) the user gets the generic message instead of something actionable.'
severity: significant
reason: 'Audited writeV1Error and writeProblemJSON call sites: every 422-emitting code path uses either the structured {detail, title, type} problem+json shape (no errors field) or {errors: []ValidationError} arrays. There is no codepath that emits a non-array errors field. The Array.isArray guard is the right narrowing for the actual server contract; defensive coding for a shape that doesn''t exist would be premature. If the server contract ever grows a non-array errors variant, the fallback''s ''detail || title || generic'' chain still surfaces a useful message via problem+json fields.'
status: wont-fix
---
