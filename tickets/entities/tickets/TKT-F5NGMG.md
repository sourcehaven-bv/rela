---
id: TKT-F5NGMG
type: ticket
title: Implement in-app configuration editing (Configure space)
kind: enhancement
priority: medium
effort: xl
started: "2026-10-04"
completed: "2026-10-05"
status: done
---

## Description

Implement the Configure mockups end to end: the Configure space in the SPA, a
backend that reads and writes `schema.yaml` and `data-entry.yaml` through a
draft → review → save flow, live reload after a save, and generated data
migrations that run as part of the save.

Mockups:
`frontend/packages/rela-components/src/pages/ConfigureDataModel.stories.ts` and
`ConfigureScreens.stories.ts`, with the fixtures `RlConfigShell`,
`RlConfigReview` and `RlConfigPreview`, and the library components
`RlSortableList`, `RlChangeList` and `RlChangeItem`.

Planning detail is in the planning checklist.
