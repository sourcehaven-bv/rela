---
id: FEAT-8OPGQ5Z
type: feature
title: Hide type and ID on data-entry cards
summary: Hide the type label and ID on data-entry cards and mention lists
description: View sections take hide_type and hide_id; entity views take mentioned_by_hide_id. Both default to false.
status: implemented
---

View sections and the "Mentioned by" panel can hide the entity type label and
the internal ID on cards and rows.

## Follow-up

Issue #2627: the data-entry guide and the `EntityViewConfig` godoc still described `entity_views.<type>.detail_view` as the canonical detail view. The `views-by-entity-type` migration strips that key on every config load, so it has no effect; an entity's page shows the view whose `entry.type` is its type. The guide and godoc now say so.
