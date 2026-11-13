---
id: RR-PWU8P8O
type: review-response
title: RunFlowWorker and RunRealtimeWorker repeat the nil-worker block
finding: Both return a closed channel for a nil worker with the same four lines.
severity: nit
reason: Four lines in two places, each on a different worker type. Folding them into workerLife would make a nil-receiver method for one line of saving.
status: wont-fix
---
