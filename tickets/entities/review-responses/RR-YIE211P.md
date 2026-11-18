---
id: RR-YIE211P
type: review-response
title: 'Design: queued records are lost on a crash'
finding: The sink appends after commit from an in-memory queue, so a crash loses every queued record, a slightly larger window than the synchronous JSONL sink.
severity: minor
reason: Closed by TKT-TU0VZ4Y, which appends inside the write transaction (DEC-U78WD7Y). The guide's Delivery section states the current behavior.
status: deferred
---
