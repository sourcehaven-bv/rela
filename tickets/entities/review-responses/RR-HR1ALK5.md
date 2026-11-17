---
id: RR-HR1ALK5
type: review-response
title: Idempotency key separator can collide
finding: The key joined fields with ':' so values containing ':' could collide.
severity: minor
resolution: The key is a JSON-encoded array. A queued run collapsing a new one for the same key is intended (one pending run per action, entity, world and caller) and is documented.
status: addressed
---
