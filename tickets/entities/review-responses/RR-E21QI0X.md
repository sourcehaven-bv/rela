---
id: RR-E21QI0X
type: review-response
title: joinDue resolves and may compile the flow on every forked write
finding: put calls joinDue, which calls flowOf, which may read and compile an older definition, on every write to a forked run with a partial arrival. Most callers already hold the compiled flow.
severity: minor
reason: A performance change to the put signature across the engine. Filed in TKT-7VAHS94. Engine is at its plimsoll method cap, so this wants a deliberate change rather than a review fix.
status: deferred
---
