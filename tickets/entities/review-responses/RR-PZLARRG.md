---
id: RR-PZLARRG
type: review-response
title: Move gate runs before the move transaction
finding: The source row can change between the gate check and the move's Tx.
severity: minor
resolution: 'Documented on checkRelationMoveMeta: the check runs before the Tx like the ACL checks in readableOrderSiblings and the Tx writes only edges the gate judged.'
status: addressed
---
