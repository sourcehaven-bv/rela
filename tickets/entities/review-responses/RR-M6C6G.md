---
id: RR-M6C6G
type: review-response
title: findListIdForEntityType iteration-order non-determinism is shipped without a guard
finding: 'findListIdForEntityType iterates Map.entries() (insertion = config-load order = JSON object key order). EntityList.vue is fine because it uses listConfig.value.detail_view from a list the user is already inside (deterministic). CustomView has no such anchor — it picks ''first list whose entity matches.'' The moment someone reorders lists in data-entry.yaml, the navigation target silently changes. Two list configs for the same type means the UX depends on YAML ordering. At minimum: document the constraint clearly. Better: introduce a ''primary'' flag on ListConfig, or a separate mapping. Best: have config-load reject ambiguity (multiple lists for same type without a primary).'
severity: critical
resolution: Resolved by config schema redesign. Adding entity_types.<type>.detail_view to data-entry.yaml as the single source of truth for 'where do you view an X entity'. Migration moves existing lists.<id>.detail_view into the new location, deduplicating per type and flagging conflicts for human reconciliation. Eliminates the 'first list wins' non-determinism entirely.
status: addressed
---
