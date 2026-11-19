---
id: AM-flow-run-list-failed-job
type: automated-measure
title: The run list shows a failed job as failed
description: A run whose job step failed for good reads failed in the run list as on its page.
kind: test
location: internal/appbuild/flow_jobs_test.go
status: active
---

TestFlowJob_RunListShowsFailedJob in internal/appbuild/flow_jobs_test.go;
TestWaitsOnFailedJob in internal/flowengine/failedjob_test.go.
