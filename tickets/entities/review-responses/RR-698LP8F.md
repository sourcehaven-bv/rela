---
id: RR-698LP8F
type: review-response
title: "refreshNow and the change event both reload"
finding: "After an action refreshNow reloads at once, and the change event can schedule the debounced reload again a second later."
severity: nit
reason: "The second load is idempotent and sticky; suppressing it would need the event to say who wrote, which it does not."
status: wont-fix
---
