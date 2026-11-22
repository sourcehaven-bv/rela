---
id: RR-AVZH1WH
type: review-response
title: Co-editing edits still spread on a read-only tenant
finding: The collab WebSocket is a GET, so other clients still receive unsaved edits on a read-only tenant. Lazy user creation on GET still writes.
severity: minor
reason: Nothing is saved, because the save is refused. Passing read-only into the collab handler is a wider change; documented in the ReadOnly godoc and the guide.
status: deferred
---
