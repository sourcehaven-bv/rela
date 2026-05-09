---
id: FEAT-KQ45P
type: feature
title: Unified entity detail screen
summary: Single config-driven screen for viewing one entity, replacing the split between EntityDetail and CustomView.
description: 'The data-entry UI has two routes that render the same conceptual thing — the detail page for a single entity — through two different code paths: /entity/:type/:id (EntityDetail.vue, hardcoded layout) and /view/:id/:entityId (CustomView.vue, config-driven). Both reimplement header, scope-nav, properties, relations, content, and actions. This feature unifies them into a single screen that always reads a ViewConfig, synthesizing a sensible default when none is defined for the entity type.'
priority: medium
status: proposed
---
