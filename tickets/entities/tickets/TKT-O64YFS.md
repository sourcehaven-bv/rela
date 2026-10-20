---
id: TKT-O64YFS
type: ticket
title: 'List toolbar_actions: entity-less action buttons on a list'
kind: enhancement
priority: medium
effort: m
status: backlog
---

## Description

A list cannot show a button for an action that runs without a selected row.
Projects work around this with sidebar items (for example "Nieuwe VvT") that
create a periodic document. Those belong on the list of generated documents.

## Approach

Add `toolbar_actions: [id, ...]` on a list. Each action renders as a button next
to "New" and runs through the entity-less `POST /_action/{id}` path, as a
navigation `action:` does. `confirm:` is honored.

The list response's `_actions` carries `action:<id>: true` for each toolbar
action whose `permission:` the principal holds. That is placement, not
authorization: the action endpoint re-checks the permission.

A list key rather than `available_on: {lists: ...}`, because `available_on`
makes an action entity-bound.
