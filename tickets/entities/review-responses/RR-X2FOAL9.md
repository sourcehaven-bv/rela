---
id: RR-X2FOAL9
type: review-response
title: history-purge reaches the erase through a free function
finding: A future HTTP or MCP history purge could forget to call PurgeDeletedJobStates.
severity: minor
reason: history-purge is CLI-only today and does not go through entitymanager; one purge seam belongs with the next purge path.
status: deferred
---
