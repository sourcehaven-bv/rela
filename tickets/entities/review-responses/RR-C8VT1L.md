---
id: RR-C8VT1L
type: review-response
title: Replay handler map must not bypass addTool
finding: 'Architecture design review: addTool returns nothing, so the plan did not say how replays reach the argument-checked handler.'
severity: minor
resolution: addTool returns the wrapped handler; registerTools fills a read-tool map that does not change after construction.
status: addressed
---
