---
id: RR-ICNI5PW
type: review-response
title: Long-running CLI watch keeps its read time
finding: channel watch binds the read time once for its whole run.
severity: minor
reason: No read-time condition depends on it yet; the expiring role grants ticket must rebind per event.
status: deferred
---
