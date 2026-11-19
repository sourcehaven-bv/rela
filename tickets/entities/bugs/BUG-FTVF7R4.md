---
id: BUG-FTVF7R4
type: bug
title: Flows panel offers Start without the start permission
description: The Flows panel on an entity page shows Start for every manual flow; a user without the start's permission gets a 403 when clicking it.
priority: medium
effort: s
why1: FlowRunsPanel offered Start for every manual flow on the type and did not know which starts the user may make.
why2: The runs-on-one-entity answer carried only runs; nothing told the client which start permissions the caller holds.
why3: The permission is checked only when the run starts on the server; the panel was built on the flow list alone.
why4: No test ran the panel for a user without the start permission.
why5: Affordances for flows are not served with the data they apply to as _actions are for entities.
prevention: TestFlowControl_Startable and TestFlowsAPI_SubjectRunsStartable pin the startable list; FlowRunsPanel.test.ts pins that Start follows it and is hidden when the runs fail to load.
started: "2026-11-19"
completed: "2026-11-19"
status: done
---

## Description

The Flows panel on an entity page shows a "Start <flow>" button for each flow
with an `on: manual` start. A user who lacks the start's `permission:` sees it
too; clicking it gives 403 "needs permission [signup.start]".

## Expected

Start shows only for a flow whose start permission the user holds on the entity.
