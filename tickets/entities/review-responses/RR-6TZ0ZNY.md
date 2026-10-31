---
id: RR-6TZ0ZNY
type: review-response
title: Group creation checked only the dragged card's permission
finding: Creating a group also creates a target entity and updates the target card. A reader who may not do either got a junk empty group before the failing write.
severity: significant
resolution: Before the first request, a new group needs update on the target card and create on the target type (the target list's _actions, now exposed as targetActions). Otherwise 'kanban.groupForbidden'. Table test covers both denials.
status: addressed
---
