---
id: RR-JMCE3S
type: review-response
title: Store error policy differs from the live relation writes
finding: The cascade now aborts on a store error while CreateRelation, UpdateRelation and DeleteRelationState continue with an empty type.
severity: minor
resolution: 'The comment at the lookup states the difference: both refuse, only the cascade reports the real cause. Aligning the live paths is left to the zero-face sweep.'
status: addressed
---
