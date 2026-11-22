---
id: AM-entity-page-refresh-routing
type: automated-measure
title: Entity page refresh, step scroll, sidebar wait, run actor, unknown routes and quiet refusals
description: Vitest tests pin that an action or an applied proposal makes the entity page's panels read their lists again, that the step form is kept in view while the page settles, that the sidebar draws no nav rows before its answer, that a system principal reads as System in the run timeline, that unknown addresses and /login resolve as intended, and that a 4xx refusal of a create logs nothing. The flow-task-hint e2e spec checks the step form is in view.
kind: test
location: frontend/src/components/entity/EntityDetail.actions.test.ts, frontend/src/components/entity/ChangesPanel.test.ts, frontend/src/utils/keepInView.test.ts, frontend/src/components/flows/focusRun.test.ts, frontend/src/components/common/Sidebar.builtins.test.ts, frontend/src/components/flows/flowView.test.ts, frontend/src/router/notFound.test.ts, frontend/src/components/forms/DynamicForm.refusal.test.ts, e2e/tests/flow-task-hint.spec.ts
status: active
---

Fails when a panel keeps its list after an action, when the step form is
scrolled once and drifts away, when the generic built-in rows flash before
the space's navigation, when the timeline shows a raw principal, when an
unknown address renders an empty pane, or when a validation refusal is logged
as an error.
