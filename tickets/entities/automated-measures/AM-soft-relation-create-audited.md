---
id: AM-soft-relation-create-audited
type: automated-measure
title: A type-mismatched relation create through the data-entry API is audited
description: TestTypeMismatchRelationWrite_Audited creates a relation whose target type the allowlist refuses (DEC-HWZHA soft condition) and asserts the 200 warning, the stored edge, and exactly one OpCreateRelation audit record with the request principal. TestCreateRelation_TolerateTypeMismatch and TestCreateRelation_TolerateTypeMismatchKeepsOwningRules pin the manager option.
kind: test
location: internal/dataentry/acl_relation_write_bypass_test.go
status: active
---
