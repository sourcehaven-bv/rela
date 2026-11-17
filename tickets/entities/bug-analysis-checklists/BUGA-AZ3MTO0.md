---
id: BUGA-AZ3MTO0
type: bug-analysis-checklist
title: 'Analysis: Flow run hidden from role-relation reader when inherit_roles_through is set'
started: "2026-11-17"
completed: "2026-11-17"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Reproduction

- [x] Bug reproduced locally (TestFlowRunReadQuery_RoleRelationWithInheritance and TestFlowControl_RoleRelationWithInheritance fail on develop 0cc979402)
- [x] Minimal reproduction steps documented (bug description)
- [x] Environment/conditions noted (any backend; acl.yaml with role_relations and inherit_roles_through)

## Root Cause

- [x] Immediate cause identified (why1)
- [x] Contributing factors found (why2-3)
- [x] Systemic cause explored (why4-5)

## Fix Planning

- [x] Fix approach determined (narrow the subject read query to direct edges before folding)
- [x] Regression test planned
- [x] Related areas checked for similar issues (related() traversals refuse inheritance loudly by design; tasks and _complete gate on the run header, so they follow the run gate)
