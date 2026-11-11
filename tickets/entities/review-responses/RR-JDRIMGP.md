---
id: RR-JDRIMGP
type: review-response
title: Header status is frozen for the whole fork; Cancel records the join
finding: The header status is set once at the fork and Cancel records the join as the cancelled state.
severity: minor
reason: The header names the join by design (it is what the task list groups by). A live status needs the worker's per-round header refresh; deferred to TKT-FWQZGAM.
status: deferred
---
