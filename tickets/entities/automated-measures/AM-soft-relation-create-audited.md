---
id: AM-soft-relation-create-audited
type: automated-measure
title: A type-mismatched relation create through the data-entry API writes nothing and audits nothing
description: TestTypeMismatchRelationWrite_RefusedUnaudited sends a relation whose target type the allowlist refuses and asserts the 422 target_type_not_allowed, no stored edge, and no write audit record (DEC-0D8UZX3; it was a DEC-HWZHA soft condition). TestCreateRelation_RefusesTypeMismatch and TestCreateRelation_TypeMismatchOnOwningEdgeRefused pin the manager refusal.
kind: test
location: internal/dataentry/acl_relation_write_bypass_test.go
status: active
---
