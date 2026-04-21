---
id: RR-HHE0R
type: review-response
title: Stale doc comment on reconcileOutgoingRelations 'nil is a no-op'
finding: Comment says 'used by PATCH requests that omit the relations key entirely, which the frontend uses for property-only saves.' DynamicForm always sends relations now (possibly empty map). The nil branch is defence-in-depth, but the 'used by' sentence is wrong.
severity: nit
resolution: Rewrote the doc comment on reconcileOutgoingRelations to describe the semantic (scoped to types present; nil/empty = no-op) without claiming a specific frontend call pattern.
status: addressed
---
