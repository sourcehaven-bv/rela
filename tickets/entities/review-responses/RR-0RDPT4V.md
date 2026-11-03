---
id: RR-0RDPT4V
type: review-response
title: Scope navigation guard compares only the id
finding: A world or ?from= change for the same entity lets an old position answer land.
severity: minor
reason: A world or scope change on one entity reloads the position for the same id; the late answer is for the same entity and is replaced by the newer one when it arrives. Ordering those needs a sequence counter; out of scope for the entity-switch bug.
status: deferred
---
