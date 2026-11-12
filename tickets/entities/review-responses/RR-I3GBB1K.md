---
id: RR-I3GBB1K
type: review-response
title: Polling waiting joins costs work per run forever and counts as progress
finding: 'Every waiting when: join costs a read, a guard evaluation and a write every 5 minutes. Above about 5000 waiting joins the poll misses its interval, and a poll that does not fire counts as Advanced and logs at Info.'
severity: minor
reason: Bounding the poll (report no-op polls apart, back off the interval while the condition stays false, reset on a change event) needs the change hook of TKT-GYXOTQC to be complete. Filed as TKT-7VAHS94. The current bound is documented and the per-tick cap keeps the worker itself bounded.
status: deferred
---
