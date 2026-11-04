---
id: TKT-8YIQEHF
type: ticket
title: Offer entity-less actions on next-action suggestions
kind: enhancement
priority: medium
effort: m
status: backlog
---

## Description

A next-action `action:` offer runs only against the suggested entity; the SPA
refuses it without an entity id. An action that creates something new (for
example Atlas's `kpi-procesuitvoering-nieuw`) cannot be offered, so "There is no
KPI measurement this month. Create one" has no button.

Found while proposing next-action config for Atlas.

## Acceptance criteria

- A suggestion without an entity (a count or query source with no candidate entity) can offer an action that takes no entity.
- Config validation rejects an entity-less action on a source whose suggestions name an entity, and the reverse, at load.
- The action runs under the caller's ACL, like a toolbar action.
