---
id: RR-PKJ0S
type: review-response
title: AC9 is factually unmet — CommandModal.vue also calls window.confirm
finding: AC9 says 'No window.confirm remains in frontend/src/'. Reviewer found CommandModal.vue:21 still calls window.confirm(cmd.confirm). The ticket says this AC is satisfied but it is not. Either fix CommandModal too, or explicitly rescope AC9 to 'delete-flow window.confirm' and document CommandModal + DynamicForm.vue:481 as known non-goals with a follow-up ticket.
severity: critical
resolution: 'Rescoped AC9 to ''delete-flow window.confirm only''. The remaining two call sites are tracked as follow-up TKT-60E9G: CommandModal.vue:21 (command confirmation before running a Lua script) and DynamicForm.vue:481 (unsaved-changes prompt in route guards). Neither is part of the delete-button UX that TKT-AYU8 targets, and the DynamicForm case specifically needs a promise-based confirm API to integrate with route guards cleanly — that is its own design problem. The ticket''s implementation checklist documents this rescope explicitly.'
status: addressed
---
