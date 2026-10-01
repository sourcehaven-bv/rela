---
id: RR-763BKE
type: review-response
title: Audit misses combinations across a global and a conferred role
finding: '[security] Views give the synthetic user one global role over NullGraph, so roles conferred per record via role_relations are never unioned in; a combination needing both is not reported.'
severity: minor
resolution: Documented as a limitation in docs/classification.md. Per-record conferred roles need an entity to resolve; evaluating every global x conferrable role pair is a follow-up if operators need it.
status: addressed
---
