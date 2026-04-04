---
id: FEAT-YIOF
type: feature
title: Interactive Lua flows with coroutine-based UI
status: proposed
description: Enable Lua scripts to collect user input through interactive forms using coroutines. Scripts yield form specifications, receive user events on resume, enabling multi-step wizards and data collection flows.
---

# Interactive Lua Flows

Enable Lua scripts to present interactive forms and collect user input, building on the existing Lua scripting capability (FEAT-i5ji).

## Motivation

Current Lua scripts are batch-oriented: they run, query data, and produce output. Many real-world workflows need user input mid-execution:

- **Entity creation wizards**: Multi-step forms that adapt based on previous answers
- **Guided data entry**: Scripts that validate and transform input before creating entities
- **Interactive reports**: Scripts that prompt for parameters before generating output

## Core Concept

Scripts use Lua's native coroutines to yield UI specifications and receive user events:

```lua
-- Script yields a form, suspends, receives event when user submits
local event = rela.flow.emit({
  type = "form",
  title = "Create Ticket",
  fields = {
    {name = "title", type = "text", required = true},
    {name = "priority", type = "select", options = {...}},
  },
  actions = {{"submit", "Create"}, {"cancel", "Cancel"}},
})

if event.action == "submit" then
  rela.create_entity("ticket", event.data)
end
```

## Design Principles

1. **Script controls flow**: The script decides what to show and when, not a declarative spec
2. **Transport-agnostic**: Same script works in CLI, web UI, or MCP (future)
3. **No side effects until commit**: Scripts collect all data first, create entities at the end
4. **Graceful cancellation**: Users can always cancel; scripts handle cleanly

## Acceptance Criteria

1. Scripts can emit forms with text, select, multi-select, boolean, number, date fields
2. Scripts can implement multi-step flows with back/cancel navigation
3. Forms render correctly in CLI using charmbracelet/huh
4. Cancellation at any point exits cleanly without creating entities

## Future Extensions

- Web transport (HTMX-based, integrate with data-entry app)
- MCP transport (AI assistants can run flows)
- `entity` field type (searchable picker with inline create)
- Wizard stdlib helpers (progress indicators, validation helpers)
