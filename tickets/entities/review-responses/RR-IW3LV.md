---
id: RR-IW3LV
type: review-response
title: Config key 'entity_types' collides with existing usage in same config
finding: 'The token ''entity_types'' is already used in commands.available_on.entity_types ([]string) and user-defaults overrides.entity_types ([]string). Adding entity_types: as a top-level map is a confusion grenade. validTopLevelKeys would also need updating. Recommendation: pick a different name (entity_views, view_routing.entity_types, or drop the new top-level entirely and put entity_types: [foo, bar] on each view (each detail view declares which types it serves)).'
severity: critical
resolution: 'Renamed config key to entity_views (avoids collision with existing entity_types: []string usages in commands.available_on and user-defaults overrides). Plan and tests updated.'
status: addressed
---
