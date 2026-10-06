---
id: RR-YGZSSR
type: review-response
title: DocumentView renders in the default world but edits in the page world
finding: renderDocument sends no world, while the edit button now opens the form in the route's world. entityId is also a bare id there.
severity: minor
reason: Document rendering per world is a separate feature. The edit form reading in the page world is the consistent choice for the entity's relations. Follow-up on the bug.
status: deferred
---
