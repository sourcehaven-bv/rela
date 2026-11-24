---
id: TKT-FGE5G3I
type: ticket
title: 'List assign: defaults for create, drop and move'
kind: enhancement
priority: medium
effort: m
started: "2026-11-24"
status: done
---

## Description

Lists get an `assign:` block: the property values and relations an item gets
when it enters the list. Used by quick-add (create prefill), by dropping a row
on the list's sidebar entry, and by keyboard move actions. Pages get the same
block for generated sidebar entries (`items_from`, `entities:`), where a
relation value `target` names the entity of the entry the row was dropped on.

```yaml
lists:
  today:
    assign:
      properties:
        when: today                       # literal
        start_date: { expr: "today()" }   # predicate value expression
pages:
  project:
    assign:
      entity_type: todo
      relations: { in_project: target }
```

- Checked at load: properties exist and are writable (not computed), literals fit enums, expressions compile with `CompileValue` against the property type and use no `related(...)`, relations exist with the item type as source and a valid target.
- `GET /api/v1/_assign?list=ID` returns resolved values for the create form prefill.
- `POST /api/v1/_assign` with `{list|page, target, entities}` writes each item through `AuthorizeUpdate` + `PatchGranted(WithRelations)` with the caller's permissions, and answers per item `{ok, error, matches}`. `matches` tells whether the item now satisfies the list's filters and condition; the UI shows a notice when false.
- SPA: sidebar entries with an assign accept row drops; rows without `_actions.update` are not draggable; the create prefill uses the list's assign.
