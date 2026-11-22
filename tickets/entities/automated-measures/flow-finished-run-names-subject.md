---
id: flow-finished-run-names-subject
type: automated-measure
title: A finished flow run read through the detail names its subject
description: TestFlowControl_FinishedRunNamesSubject reads a cancelled run through Run and SubjectRuns as two readers of the subject and one non-reader.
kind: test
location: internal/appbuild/flowsteps_test.go (TestFlowControl_FinishedRunNamesSubject)
status: active
---

Fails when the run detail takes the subject only from the run store, which drops
a run once it finishes.
