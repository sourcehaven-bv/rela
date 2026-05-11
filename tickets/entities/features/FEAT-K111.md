---
id: FEAT-K111
type: feature
title: Entity views are strictly read-only
summary: Entity detail views render relations as read-only sections; mutations happen exclusively through forms.
description: Restore the conceptual separation between viewing and editing in data-entry. The detail screen for an entity (rendered by EntityDetail.vue / GET /api/v1/entities/{type}/{id}/view) shows the entity's relations as read-only sections — no inline +Add or Link Existing affordances. All graph mutations happen through the form path (DynamicForm + SidePanel), where mutation affordances are appropriate.
priority: medium
status: proposed
---
