---
id: flow-lists-name-subject
type: automated-measure
title: Flow lists name the subject by its title
description: The flow run list and task list and run page name each subject by its display title or by its id when a title property is hidden from the caller.
kind: test
location: internal/appbuild/flowsteps_test.go
status: active
---

TestFlowControl_SubjectTitle and TestFlowControl_SubjectTitleHidden in
internal/appbuild/flowsteps_test.go; TestWholeDisplayTitle in
internal/metamodel; frontend/src/views/FlowLists.test.ts.
