---
id: RR-HSZX07I
type: review-response
title: Fired faces depend on what the caller can read
finding: The same edge write gives different results per caller.
severity: minor
reason: Each written face now needs update permission, so a caller only changes rows it may change.
status: wont-fix
---
