---
id: RR-OPGU3N7
type: review-response
title: Writable peer can poison the saved-hash map
finding: A writable peer can add any hash to the room's saved map and make the saver resend over an external edit. That peer can already overwrite the body directly with the same permission, so no privilege is gained.
severity: minor
reason: The peer already has update permission and can overwrite the body directly; no privilege gained.
status: wont-fix
---
