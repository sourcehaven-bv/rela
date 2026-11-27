---
id: RR-BBAQ6GC
type: review-response
title: Row may change beyond the toggled field
finding: A trigger can set other properties (for example a completed date) on the PATCH. An optimistic flip alone leaves the row stale.
severity: significant
resolution: The SPA replaces the row with the PATCH response after success and reverts the flip on error.
status: addressed
---
