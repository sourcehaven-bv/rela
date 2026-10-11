---
id: RR-1PWL4P
type: review-response
title: Incoming densify writes edges of other sources
finding: 'An edge belongs to its source: UpdateRelation authorizes the moved edge''s source only. On the incoming side the siblings have different sources, so a densify would rewrite edges the caller may not update. Authorize an update of every candidate sibling before the transaction, write only authorized edges, and gate the _order_in meta affordance over every sibling source in the handler.'
severity: significant
resolution: 'moveRelation calls authorizeIncomingSiblings before the Tx: it authorizes RelationUpdateRequest for every edge the move may write and the Tx writes only those. TestUpdateRelation_IncomingPositionAuthorizesSiblings pins that one denial writes nothing. The handler gates _order_in on every sibling source too, and movable answers false when any source is denied.'
status: addressed
---
