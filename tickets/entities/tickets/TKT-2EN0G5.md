---
id: TKT-2EN0G5
type: ticket
title: Scoped relation-picker candidates and a working FormRelation default
kind: enhancement
priority: medium
effort: m
status: backlog
---

## Description

Restrict relation-picker candidates and make a relation `default` work. Part of
RES-8CKUNJ.

Today `RelationPicker` offers every entity of the target type
(`RelationPicker.vue:238-300`). The SPA reads `field.default` for relations
(`DynamicForm.vue:845-851`), but the Go `FormRelation` struct has no `default`
key, so a configured default is silently dropped.

## Scope

- `FormRelation.candidates`: limit options to the targets of a relation on
the page anchor (for example `offered_by: offers_status`), or to a
`query_scope`.
- `FormRelation.default`: a target id, or `first` of the scoped candidates.
Config load rejects unknown keys instead of dropping them.
- The same candidate scoping applies to relation `filter_controls`.

## Acceptance criteria

- Creating a task from an initiative page offers only that initiative's
statuses and preselects the first.
- A `default` on a form relation is either applied or rejected at load.
