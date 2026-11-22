---
id: AM-flow-step-blocked-message
type: automated-measure
title: A step that no branch accepts says so
description: Completing a step when no open change branch's guard holds answers problem blocked and the run card warns.
kind: test
location: internal/appbuild/flowsteps_test.go
status: active
---

TestFlowControl_CompleteStep in internal/appbuild/flowsteps_test.go;
TestEngine_Change in internal/flowengine/person_test.go; FlowRunCard.test.ts in
frontend/src/components/flows.
