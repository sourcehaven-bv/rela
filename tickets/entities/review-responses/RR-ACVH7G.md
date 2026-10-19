---
id: RR-ACVH7G
type: review-response
title: Implicit channels created by triggers make entities undeletable under `block`
finding: 'Risk 6 only documents that a trigger''s implicit channel turns on `on_entity_delete: block` for every matching entity. An operator who adds a notification trigger silently changes delete semantics for thousands of entities. Refuse at load a trigger whose channel type uses `block`, or say which of the two behaviours is intended, with an AC.'
severity: significant
resolution: 'Per Jeroen: allowed and documented in docs/realtime.md under on_entity_delete.'
status: addressed
---
