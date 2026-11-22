---
id: AM-flow-failure-step-and-panel-gate
type: automated-measure
title: Failure step in the run log and the flow runs panel gate
description: Engine tests pin that a job that fails for good adds one ViaFailed step at its state; handler and config tests pin that entity_panels.flow_runs withholds the subject runs per space and per permission.
kind: test
location: internal/flowengine/job_test.go, internal/dataentry/flows_handler_test.go, internal/dataentryconfig/chrome_test.go
status: active
---

Fails when a failed job leaves the timeline without an entry, adds one more
than once, or when a hidden flow runs panel still gets its runs.
