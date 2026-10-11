---
id: RR-J7A5EV
type: review-response
title: Incoming order visibility probed on the first source row only
finding: 'newRelationOrdering checked _order_in visibility against g.sources[0].row. For an identity-scoped relation a faced source yields one row per readable face, and a visible: grant resolved on another face was never consulted.'
severity: minor
resolution: The check now runs on every source row of every group; one hidden verdict turns the order off.
status: addressed
---
