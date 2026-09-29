---
id: RR-90WO4Y
type: review-response
title: Sync ApplyEntity does not validate file keys
finding: A synced value may reference a key the entity has no bytes for, or another entity's key shape.
severity: minor
reason: Sync is an operator-trusted channel; a key without bytes only yields a 404, and keys are scoped per entity and property on disk. Revisit with the Stage 3 id work.
status: deferred
---
