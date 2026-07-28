---
id: RR-XCAI6J
type: review-response
title: visibleRelationMeta doc comment overstates 'always returns a fresh map'
finding: The doc comment claimed the helper 'always returns a fresh map (never the argument)', but on the no-op branches (resolver not a RelationVisibilityResolver, empty meta, or nothing hidden) it returns the input map (store-owned edge.Properties) unchanged. Safe today because callers only read the result, but the comment would mislead a future caller who trusts 'never the argument' and mutates it.
severity: nit
resolution: 'Corrected the comment: ''returns a fresh copy only when at least one key is redacted; returns the input unchanged when nothing is redacted; callers must treat the result as read-only.'''
status: addressed
---
