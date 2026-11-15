---
id: RR-ZXPYDQJ
type: review-response
title: max ignores relations the entity already has
finding: The proposal max is not lowered by the number of existing relations of that type.
severity: minor
reason: max bounds the size of one proposal; the relation's max_outgoing is enforced by the write path when the user applies it, and the dialog reports a refused relation. Narrowing further is a follow-up.
status: deferred
---
