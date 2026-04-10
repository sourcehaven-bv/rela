---
id: TKT-WEAU1
type: ticket
title: Add built-in scheduled task runner for Lua scripts
kind: enhancement
status: done
priority: medium
effort: l
---

## Description

Rela should support running scheduled tasks (Lua scripts) without depending on
external cron or task scheduling infrastructure. This allows projects to define
recurring automation — such as periodic analysis, report generation, or data
validation — as Lua scripts that rela executes on a configurable schedule.

## Acceptance Criteria

- A new CLI command (e.g. `rela scheduler`) starts a long-running process that executes Lua scripts on configured schedules
- Schedules are defined in a project configuration file (e.g. `schedules.yaml` or within `metamodel.yaml`)
- Each scheduled task references a Lua script in the project's `scripts/` directory
- Supports cron-like schedule expressions
- Tasks have access to the same Lua runtime capabilities as `rela script` (entity CRUD, graph queries, AI, etc.)
- Graceful shutdown on SIGINT/SIGTERM
- Logging of task execution (start, completion, errors)
- Overlapping execution prevention (skip if previous run still active)
- Missed run detection: on startup, check each task's last-run timestamp (persisted in `.rela/scheduler-state.json`); if a scheduled window was missed while the scheduler was not running, execute immediately
- State file (`.rela/scheduler-state.json`) persists last successful run time per task
