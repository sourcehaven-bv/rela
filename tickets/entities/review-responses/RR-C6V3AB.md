---
id: RR-C6V3AB
type: review-response
title: 'Ephemeral tier: active runs from a dead process wait out their lease'
finding: On fs/desktop/sqlite the queue is in memory, so an active run found at startup belongs to a dead process, yet the task waits 20-30 minutes and records a failure.
severity: minor
reason: 'Accepted trade-off, reported to the user: the ephemeral queue loses jobs on exit by design, and a restart rarely falls inside a run. A startup sweep would need the scheduler to know whether its queue is durable, which it deliberately does not. Revisit if desktop users report delayed tasks after restarts.'
status: deferred
---
