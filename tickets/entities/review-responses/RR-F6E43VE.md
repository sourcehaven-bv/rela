---
id: RR-F6E43VE
type: review-response
title: FlowWorkerTick races with a running worker
finding: The test hook calls runTick from the test goroutine and would race on w.after if a test also started the worker.
severity: nit
resolution: The hook's doc says the worker must not be running, and runTick's doc says only the leading goroutine calls it.
status: addressed
---
