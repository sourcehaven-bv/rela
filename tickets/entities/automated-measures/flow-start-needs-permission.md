---
id: flow-start-needs-permission
type: automated-measure
title: Flows panel offers Start only with the start permission
description: The runs of one entity name the flows the caller may start there and the panel offers Start only for those.
kind: test
location: internal/appbuild/flowsteps_test.go
status: active
---

TestFlowControl_Startable in internal/appbuild/flowsteps_test.go;
TestFlowsAPI_SubjectRunsStartable in internal/dataentry;
frontend/src/components/flows/FlowRunsPanel.test.ts.
