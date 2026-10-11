---
id: RR-1BN25K
type: review-response
title: An add racing a delete leaves an unresolvable item
finding: An add resolving X before X is deleted lands after the EntityDeleted hook ran, so X stays on the pile invisibly, uses a slot, and reappears if the id is reused. Same for deletes made outside rela on fs.
severity: minor
reason: 'The stale item is invisible (read path filters through the visibility reader) and only costs a slot. Pruning on read needs a write on the read path or a sweep; deferred to a follow-up. Id reuse resurfacing an item is accepted for now: the item re-appears only if the principal can read the new entity.'
status: deferred
---
