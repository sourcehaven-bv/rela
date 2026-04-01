---
description: 'List columns with relation: currently only show outgoing relations. Add direction:incoming support to display entities that point to the current row.'
id: FEAT-0c9l
priority: medium
status: implemented
summary: Show incoming relations in list columns via direction:incoming config
title: Support direction:incoming for relation columns in lists
type: feature
---

## Use Case

In a roles list, show "Held By" (which persons hold this role) and "Controls" (which controls are owned by this role). The relations are:
- hasRole: person → role
- controlOwnedByRole: control → role

Both point to the role, so incoming edges are needed.

## Solution

Add `Direction` field to `ListColumn` struct and update `resolveRelationColumnValues()` to use `IncomingEdges()` when `direction: incoming`.

## Example Config

```yaml
columns:
  - relation: hasRole
    direction: incoming
    label: 'Held By'
  - relation: controlOwnedByRole
    direction: incoming
    label: 'Controls'
```
