---
id: RR-1IGO1
type: review-response
title: schemaGraphvizFixture doc comment references nonexistent `bs` parameter
finding: Comment says 'Pass bs as nil to keep all rendering flags at their default' but the signature is `(t, ents, rels)`. Stale from an earlier iteration.
severity: nit
resolution: 'Rewrote the doc comment to describe current behavior: ''All rendering flags start at their defaults — tests can override them after calling this helper.'''
status: addressed
---
