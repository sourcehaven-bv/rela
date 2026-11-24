---
id: TKT-30FRA9A
type: ticket
title: 'Data-entry config fixes: one colour list, board condition, reserved keys, select widget error'
kind: enhancement
priority: medium
effort: m
started: "2026-11-24"
completed: "2026-11-24"
status: done
---

## Description

Four data-entry.yaml loading fixes (gap 19).

1. One colour list: `amber` is valid for list groups and calendars but not for `style:`. Make one shared list of colour names, with aliases, and map every name in the SPA.
2. Allow `condition:` on a kanban board through the same read path lists use (`kanban_id` next to `list_id`).
3. Document the reserved list keys `j k o e n h l`, and name the key and the action in the load error.
4. An explicit `widget: select` or `widget: multi-select` on a form property field fails at load. The message names the form and field, says to remove the line, and says what the form shows instead.
