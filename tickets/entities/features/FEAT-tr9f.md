---
description: 'List columns with relation: currently only show outgoing relations. Add support for direction: incoming to display entities that point to the current row.'
id: FEAT-tr9f
status: implemented
title: 'Support direction: incoming for relation columns in lists'
type: feature
---

## Use case

In a roles list, showing "Held By" (which persons hold this role) and "Controls" (which controls are owned by this role). The relations are:
- hasRole: person → role
- controlOwnedByRole: control → role

Both point to the role, so incoming edges are needed.

## Solution

Add `Direction` field to `ListColumn` struct and update `resolveRelationColumnValues()` to use `IncomingEdges()` when `direction: incoming`.

## Config example

```yaml
columns:
  - relation: hasRole
    direction: incoming
    label: 'Held By'
```
