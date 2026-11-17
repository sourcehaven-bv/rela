---
id: AM-run-gate-role-relation-with-inheritance
type: automated-measure
title: 'Regression test: a role-relation reader reads the run with inherit_roles_through set'
description: TestFlowRunReadQuery_RoleRelationWithInheritance checks that a direct role-relation edge on the subject admits the run, an inherited-only subject hides it, and who-can credits no ancestor route. TestFlowControl_RoleRelationWithInheritance checks run view, subject runs, tasks and complete for that reader (BUG-A7QDXU1).
kind: test
location: internal/acl/managed_test.go and internal/appbuild/flowsteps_test.go
status: active
---
