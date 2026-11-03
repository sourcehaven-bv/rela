---
id: RR-4TY69BA
type: review-response
title: A never-settling write blocks the queue
finding: All writes share one promise chain; a write without a timeout blocks every later save.
severity: nit
resolution: Documented on write() and at the acceptComment call site.
status: addressed
---
