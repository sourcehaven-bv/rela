---
id: graph-dot-hyphen-sanitization-test
type: automated-measure
title: 'Test: DOT subgraph cluster IDs are sanitized for hyphenated entity types'
description: Unit test `TestGenerateDOT_HyphenatedEntityType` asserts that when the metamodel contains an entity type with a hyphen (e.g. `review-response`), `generateDOT` emits `subgraph cluster_review_response` and never `cluster_review-response`. Paired with `TestSanitizeDOTID` (table) that pins the sanitization rules. Prevents BUG-graph-hyphen from recurring.
kind: test
location: internal/cli/graph_test.go
status: active
---
