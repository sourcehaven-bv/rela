---
id: AM-flow-finished-run-timeline
type: automated-measure
title: A finished flow run keeps its timeline
description: Pins that a completed or cancelled run keeps its log in the header body, that a manager sees the timeline, the diagram at the final state and the manage view, that the run list carries no timeline, and that a missing or unreadable log does not fail the page.
kind: test
location: internal/appbuild/flowsteps_test.go
status: active
---

TestFlowControl_FinishedRunKeepsTimeline and
TestFlowControl_FinishedRunWithoutLog in internal/appbuild/flowsteps_test.go;
TestLogBody in internal/flowrun; FlowRunCard.test.ts and flowView.test.ts in
frontend/src/components/flows.
