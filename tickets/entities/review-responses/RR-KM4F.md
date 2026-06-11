---
id: RR-KM4F
type: review-response
title: entity.page.ts clickContentCheckbox docstring lies about post-TKT-R7Q9 mechanism
finding: Docstring says 'The Vue handler calls preventDefault() then reloads the view'. Post-TKT-R7Q9 it's PATCH + reactive entity mutation, no reload. Page-objects are the closest thing to test docs; stale comments rot trust.
severity: minor
resolution: Updated docstring on entity.page.ts:clickContentCheckbox to describe the post-TKT-R7Q9 PATCH + reactive splice flow (instead of the pre-fix preventDefault + reload). Mentions no-flicker contract as the why.
status: addressed
---
