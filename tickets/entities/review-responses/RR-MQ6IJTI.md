---
id: RR-MQ6IJTI
type: review-response
title: '[security] Deleted id of another type served via relation records'
finding: '[security] A history:read holder could ask for a deleted id under a readable type and see relation records of that id from a readable neighbor, even if the id was of another type.'
severity: significant
resolution: 'EntityChanges reads the lifecycle start record and serves a list only when it started an entity of the asked type at the asked face, or when there is none and the entity is live. Tests: TestEntityChanges_StartFence and TestHandleV1Changes_DeletedIDOfAnotherType.'
status: addressed
---

[security] A history:read holder could ask for a deleted id under a readable
type and see relation records of that id from a readable neighbor, even if the
id was of another type.
