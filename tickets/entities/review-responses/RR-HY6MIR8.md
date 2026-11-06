---
id: RR-HY6MIR8
type: review-response
title: Room outlives its entity
finding: A room is kept 5 minutes after it empties. Deleting an entity and recreating it with the same id lets a new editor join the old document.
severity: minor
reason: The 30 second recheck closes every connection once the entity is gone, and an idle room is dropped after five minutes. A delete hook can come later if the window matters.
status: deferred
---
