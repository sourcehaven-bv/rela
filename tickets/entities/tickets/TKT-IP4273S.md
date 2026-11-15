---
id: TKT-IP4273S
type: ticket
title: Kanban quick-add
kind: enhancement
priority: medium
effort: m
started: "2026-11-15"
status: in-progress
---

## Description

A facilitator types a title straight into a kanban column, or into a column by
swimlane cell, and presses Enter. That creates an entity with the column value
(and the swimlane value) set, without opening the full form. It is optional per
board (`quick_add: true` in `data-entry.yaml`) and off by default.

Part of the DPIA workshop follow-ups to TKT-35BY3PE.

- Writes go through the existing create API and `entitymanager.Manager` (ACL, validation, audit).
- Validation errors show inline.
- The control is hidden when the principal may not create the type (`_actions.create`).
- Required properties and entity templates are handled as recorded in the planning checklist.
