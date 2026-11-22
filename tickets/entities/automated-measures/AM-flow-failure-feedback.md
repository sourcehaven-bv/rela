---
id: AM-flow-failure-feedback
type: automated-measure
title: Failed flow runs and step forms give their feedback
description: Go tests pin the job error for managers only, the failed tasks, the rule fields and pattern warnings of a step, failed_message and the run title; frontend tests pin the inline field feedback, the blocked notice, the retrying line and the failed text.
kind: test
location: internal/appbuild/flow_jobs_test.go, internal/appbuild/flowsteps_test.go, internal/flowcfg/title_test.go, internal/writerule/writerule_test.go, internal/flowengine/job_test.go, frontend FlowRunCard.test.ts, FieldShell.test.ts, FlowLists.test.ts
status: active
---

Fails when a reader who does not manage a run receives its job error, when
Flow tasks stops listing failed runs, when a rule refusal or a pattern warning
of a step no longer names its field, when a step form shows feedback as a
toast, or when a run's stored title copies the subject.
