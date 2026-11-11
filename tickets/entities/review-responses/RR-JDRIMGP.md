---
id: RR-JDRIMGP
type: review-response
title: Header status is frozen for the whole fork; Cancel records the join
finding: The header status is set once at the fork and Cancel records the join as the cancelled state.
severity: minor
reason: The header names the join by design (it is what the task list groups by). A live per-branch status is UI and header work that the worker does not need; moved from TKT-FWQZGAM to TKT-YNQPHPP.
status: deferred
---
