---
id: FEAT-W7CM2A
type: feature
title: Relation-backed status
summary: A task's workflow state is a single-valued relation to a status entity. Boards and lists derive columns and sections from it; views read status properties through typed relation paths.
description: 'Atlas wants a board per initiative with its own ordered sections. Today kanban columns come from one enum per entity type. With this feature a task links to exactly one status entity; a parent selects and orders the statuses it offers through an orderable relation. Boards and grouped lists take their columns and sections from that relation. Views read status properties (name and colour and order and category) through relation paths such as has_status.kleur that are resolved at read time. Design: RES-8CKUNJ. Follow-ups: TKT-65LVAK TKT-DA9C0L TKT-KJ3Q07 TKT-JO8PN3 TKT-2EN0G5 TKT-ZKPA1E.'
priority: medium
status: proposed
---
