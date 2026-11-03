---
id: AM-bounded-relation-replace-audited
type: automated-measure
title: A type-mismatched create on a bounded relation through the data-entry API is audited
description: TestPatchRelations_DisallowedTypeOnBoundedRelationAudited PATCHes a bounded relation with a create the type allowlist refuses (DEC-HWZHA soft condition) and asserts the 200 warning, the stored edges, and an audit record with the request principal for every create and remove. TestPatchRelations_DisallowedTypeOverBoundIs422 and TestReplaceRelations_TolerateTypeMismatch pin that the bound still applies and the manager option.
kind: test
location: internal/dataentry/relations_cardinality_test.go
status: active
---
