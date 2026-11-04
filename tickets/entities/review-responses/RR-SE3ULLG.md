---
id: RR-SE3ULLG
type: review-response
title: No capacity pre-check before the entity writes
finding: CreateLinked checks existing-end capacity before writing the entity, so a full bound writes nothing on fs. CreateBatch writes all entities first.
severity: minor
resolution: recheckEnds runs checkLinkedRelation (capacity) for every existing end before any entity is written inside the Tx.
status: addressed
---
