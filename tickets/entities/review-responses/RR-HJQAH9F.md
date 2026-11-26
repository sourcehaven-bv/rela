---
id: RR-HJQAH9F
type: review-response
title: A run started by this write also gets its event
finding: List inside the Tx sees a run a predicate start just created.
severity: minor
resolution: Runs started in the same call are excluded from change events.
status: addressed
---
