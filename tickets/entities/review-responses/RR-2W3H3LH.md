---
id: RR-2W3H3LH
type: review-response
title: A cancelled or busy POST is not held as uncertain
finding: pushCreate returned the run error for a cancelled context or a RefusedError before unknownCreate ran, so a create that reached the remote was sent again.
severity: critical
resolution: Every POST error except a refused credential or a dial failure holds the row as uncertain before the run error returns. TestPush_CreateMayHaveReachedTheRemote.
status: addressed
---
