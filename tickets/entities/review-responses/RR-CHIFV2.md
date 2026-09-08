---
id: RR-CHIFV2
type: review-response
title: New batching helpers must be free functions, not App methods (plimsoll cap)
finding: app.go:172 pins //plimsoll:max-methods=87 while worldneighbors.go:85 cites 104; one is stale. The batching rewrites and the Services() hoist will want helpers; adding them to App breaks the cap.
severity: minor
resolution: Plan states all new batching helpers are free functions taking their read seams (the visibleRelationIDs shape); the 87-vs-104 discrepancy is reconciled in the first PR.
status: addressed
---
