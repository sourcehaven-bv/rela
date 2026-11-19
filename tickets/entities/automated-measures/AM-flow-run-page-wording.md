---
id: AM-flow-run-page-wording
type: automated-measure
title: The run page names the flow once and actors by title
description: Pins the actor title on the timeline (gated and only for mapped persons), the singular and plural failed-attempt text, no no-roles line on a failed job, the hidden card title on the run page and the space home row highlight.
kind: test
location: internal/appbuild/flowsteps_test.go
status: active
---

TestFlowControl_TimelineNamesActor in internal/appbuild/flowsteps_test.go;
flowView.test.ts and FlowRunCard.test.ts in frontend/src/components/flows;
sidebarNav.test.ts in frontend/src/components/common.
