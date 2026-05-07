---
id: RR-DF3QG
type: review-response
title: YAML insertion location for new top-level key not specified
finding: 'yaml.Node mapping append puts new keys at end of file. Decide: insert near lists: (semantic neighbor) or at top after version:/app: (metadata). Need a yaml_util helper InsertMapKeyAfter(anchor, key, value). One-time decision, hard to reverse for users.'
severity: minor
resolution: 'Decision: insert entity_views: after lists: (or after forms: if lists is absent) using a new yaml_util.go helper InsertMapKeyAfter(root, anchor, key, valueNode). Rationale: entity_views is conceptually adjacent to lists/views and reads naturally there.'
status: addressed
---
