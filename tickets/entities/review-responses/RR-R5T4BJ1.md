---
id: RR-R5T4BJ1
type: review-response
title: An edge to an unreadable target still counts for the UX gates
finding: HasOutgoingEdge does not check that the target entity is readable. A principal can see a gated menu entry because of an edge to an entity they cannot read.
severity: minor
reason: The gates are UX filters over config that is not secret. Showing an entry leaks no entity content and every read behind it stays gated. Conferring edges are identity-scoped so the edge itself is the principal's own grant.
status: wont-fix
---
