---
id: AM-bounded-relation-replace-audited
type: automated-measure
title: A type-mismatched create on a bounded relation through the data-entry API writes nothing
description: TestPatchRelations_RejectsDisallowedEndpointTypes PATCHes bounded and unbounded relations with a create the type allowlist refuses and asserts the 422, unchanged edges and properties, and no audit record (DEC-0D8UZX3; it was a DEC-HWZHA soft condition). TestPatchRelations_DisallowedTypeOverBoundIs422 and TestReplaceRelations_RefusesTypeMismatch pin the refusal on an over-bound add and in the manager.
kind: test
location: internal/dataentry/relation_type_contract_test.go
status: active
---
