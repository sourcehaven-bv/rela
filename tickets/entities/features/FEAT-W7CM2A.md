---
id: FEAT-W7CM2A
type: feature
title: Relation-backed status
summary: A task's workflow state is a single-valued relation to a status entity. Boards and lists derive columns and sections from it; lookup properties keep property-driven features working.
description: 'Atlas wants a board per initiative with its own ordered sections. Today kanban columns come from one enum per entity type. With this feature a task links to exactly one status entity; a parent selects and orders the statuses it offers through an orderable relation. Boards and grouped lists take their columns and sections from that relation. Lookup properties copy the status category onto the task so that styles and state machines and CalDAV and search keep working on a plain property. Design: RES-8CKUNJ. Follow-ups: TKT-65LVAK TKT-DA9C0L TKT-KJ3Q07 TKT-JO8PN3 TKT-2EN0G5 TKT-ZKPA1E.'
priority: medium
status: proposed
---
