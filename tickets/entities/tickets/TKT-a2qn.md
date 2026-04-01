---
effort: m
id: TKT-a2qn
kind: enhancement
priority: medium
status: done
title: Add automatic entity creation to automation engine
type: ticket
---

## Description

The automation engine currently supports property changes (set) and relation creation (create_relation) as actions. This ticket adds support for automatic entity creation as a new action type.

Use case: When a ticket transitions to 'in-progress', automatically create a planning-checklist entity and link it to the ticket.
