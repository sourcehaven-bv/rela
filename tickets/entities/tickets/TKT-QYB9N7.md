---
id: TKT-QYB9N7
type: ticket
title: Collapsible kanban columns
kind: enhancement
priority: medium
effort: s
started: "2026-10-09"
completed: "2026-10-09"
status: done
description: Let a reader collapse a kanban column to a narrow rail with title and count, remembered per board; columns[].collapsed sets the default.
---

## Description

A kanban column cannot be collapsed. Boards with a parking column (Atlas
"Postpone", Atlas TASK-8997C) take a full column width for cards nobody is
looking at.

The component library already renders a collapsed column
(`RlBoardColumnCollapsed`, and the rail in `RlSwimlaneBoard`) and emits
`expandSection`. What is missing:

- a control on the column heading to collapse it;
- KanbanView wiring that marks a section `collapsed` and handles expand/collapse;
- remembering the reader's choice per board (localStorage, like grouped list sections);
- a config default: `columns[].collapsed: true` starts that column folded.

## Acceptance criteria

- A reader can collapse and expand any column on a plain and a swimlane board.
- A collapsed column shows its title (vertical) and its card count.
- The choice survives a reload, per board, per browser.
- `collapsed: true` on a declared column starts it collapsed; a reader can still expand it.
- `collapsed:` on an inferred column is impossible (only declared columns carry it); docs say so.
- e2e covers collapse, reload persistence and the config default.
