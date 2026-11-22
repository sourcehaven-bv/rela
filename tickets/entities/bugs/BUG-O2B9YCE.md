---
id: BUG-O2B9YCE
type: bug
title: Flow run lists show subject ids and unstyled links
description: The run list and the task list show each subject by its id instead of its title; their links use the browser default blue underline.
priority: medium
effort: s
why1: The run and task views carry only the subject address, so the lists and the run page print the id.
why2: FlowControl built run views from run headers and the run store, and never read the subject header for its title.
why3: The lists were built before the flow views named subjects, and the tables used plain RouterLinks without the app link class.
why4: No test compared the flow lists with other tables, which show titles and the accent link style.
why5: A list that shows entities has no shared rule or helper that names them by title.
prevention: TestFlowControl_SubjectTitle and TestFlowControl_SubjectTitleHidden pin the title on every flow view; FlowRunsView.test.ts pins the title and the link class in both tables.
started: "2026-11-19"
completed: "2026-11-19"
status: done
---

## Description

The flow run list (`/flows/runs`) and the task list (`/flows/tasks`) show the
Item column as the subject id, such as `ACC-001`, instead of its title. The run
page says "Open ACC-001". The links in both tables have the browser default blue
underline, not the app link style used in other tables.

## Expected

Each subject shows its display title, falling back to its id. Links use the
accent colour without an underline, underlined on hover.
