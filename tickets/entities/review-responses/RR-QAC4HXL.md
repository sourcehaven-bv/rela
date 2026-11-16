---
id: RR-QAC4HXL
type: review-response
title: The reach is keyed by id, not by face
finding: With a faced row type, an edge landing on a face the reader cannot read still admits another readable face of the same id. The list tab has the same behaviour today.
severity: minor
reason: Pre-existing in resolvePageScope for list tabs; fixing it belongs with the page scope, not this ticket. Card counts reuse the same rule, so both stay consistent.
status: deferred
---
