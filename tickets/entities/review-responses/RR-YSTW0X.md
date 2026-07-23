---
id: RR-YSTW0X
type: review-response
title: Relation cells silently drop neighbors on store error (inherited)
finding: outgoingRelations/incomingRelations swallow store errors (log + return partial slice, documented TKT-N26KLB). On export a transient store failure yields a silently-incomplete relation cell rather than a failed export; the truncation notice covers row-count but not dropped neighbors. Export is a capture-of-record surface where silent partial data is worse.
severity: minor
reason: Inherited behavior of the entityReader read seam (TKT-N26KLB), not introduced by this ticket, and affects every consumer of outgoingRelations/incomingRelations (list view, entity view, serializer) equally. Fixing it belongs in that seam, not bolted onto export. Tracked as a follow-up against the entityReader error-handling gap rather than blocking this feature.
status: deferred
---
