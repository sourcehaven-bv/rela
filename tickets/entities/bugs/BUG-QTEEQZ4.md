---
id: BUG-QTEEQZ4
type: bug
title: GET /_flows answers 404 on every page of a project without flows
description: 'A project without flows.yaml logs a 404 for GET /api/v1/_flows on every page load and on every entity page. The SPA asked for the flow list without knowing whether the project has flows.'
priority: low
effort: xs
why1: 'useFlowList asks for GET /_flows when the sidebar mounts and again on each entity page after a failure, and a project without flows answers 404.'
why2: 'The SPA had no way to learn that a project has no flows before asking; useFlowList used the 404 as the signal for no flows.'
why3: 'When flows were added (TKT-ZAXRXND) the routes took the nil-handler 404 of the other optional surfaces, but the surface was not advertised in /_schema the way capabilities.git is.'
why4: 'No rule says that an optional surface the SPA calls on every page is advertised in /_schema so the SPA can skip it.'
why5: 'Optional API surfaces were added one by one, each picking its own way to say it is absent (404, piles_available, capabilities.git), with no shared convention for what the SPA may call unconditionally.'
prevention: '/_schema reports capabilities.flows and useFlowList asks only when it is true. A Go test pins the capability and a vitest pins that the list is not asked for without flows.'
started: "2026-11-21"
completed: "2026-11-21"
status: done
---

## Description

On a project without `flows.yaml`, every page logs `GET /api/v1/_flows 404`
in the browser console. The sidebar reads the flow list to decide whether to
show the flow entries, and the entity page reads it to decide whether to show
flow runs. The server answers 404 because no flow service is wired, and
`useFlowList` treated a failure as "no flows" and asked again on the next use.

## Acceptance Criteria

- [x] `GET /api/v1/_schema` reports `capabilities.flows`: true when the project declares flows, false otherwise.
- [x] The SPA does not call `GET /api/v1/_flows` when `capabilities.flows` is false.
- [x] A project with flows behaves as before.
- [x] The capability is documented.
