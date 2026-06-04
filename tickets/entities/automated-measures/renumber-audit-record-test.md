---
id: renumber-audit-record-test
type: automated-measure
title: 'Test: renumber writes emit audit records'
description: 'Regression test for BUG-Q7GYJ. TestRenumber_EmitsAuditRecords collapses sibling order spacing so an UpdateRelation triggers maybeRenumberSide, then asserts the cascaded renumber writes emit update-relation audit records carrying a renumber: triggered-by marker — distinct from the user-initiated update. Guards against renumber writes silently bypassing the audit log again. The CLI path (rela renumber) is covered structurally by routing through entitymanager.Manager, which all audited write paths use.'
kind: test
location: internal/entitymanager/orderable_test.go (TestRenumber_EmitsAuditRecords)
status: active
---
