---
id: conflict-endpoint-containment-test
type: automated-measure
title: 'Test: conflict endpoints contain paths and re-authorize writes'
description: 'Regression tests for BUG-JME1DI: GET/POST conflict endpoints reject relative, deep-relative, and absolute path traversal (403, no content leak, no write outside the project root); a denied resolve returns the structured 403 ACL body, leaves the file untouched, and records a denied-write audit row; successful entity and relation resolves remove markers and record update-entity/update-relation audit rows.'
kind: test
location: internal/dataentry/conflicts_api_test.go (TestV1ConflictDetailPathTraversal, TestV1ConflictResolvePathTraversal, TestV1ConflictResolveACLDenied, TestV1ConflictResolveEntityWritesAndAudits, TestV1ConflictResolveRelationWritesAndAudits) + internal/dataentry/lint_test.go (TestNoStrayWriteRequestConstruction covers translateRelationWrite)
status: active
---
