---
id: BUGA-J0BQ3TQ
type: bug-analysis-checklist
title: 'Analysis: Conflict resolve skips field rights and errors quote hidden values'
started: "2026-11-05"
completed: "2026-11-05"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Reproduction

- [x] Bug reproduced locally (TestV1ConflictResolve_FieldWriteGate, TestV1ConflictResolve_RelationMetaWriteGate, TestV1ConflictResolve_ValidationErrorHidesValue, TestV1UpdateEntity_WarningHidesHiddenValue, TestGitStatusResponse_ConflictFiles all fail on origin/develop)
- [x] Minimal reproduction steps documented (in the tests and the bug body)
- [x] Environment/conditions noted (fs store with git conflicts; any store for the validation message leak)

## Root Cause

- [x] Immediate cause identified (why1)
- [x] Contributing factors found (why2-3)
- [x] Systemic cause explored (why4-5)

## Fix Planning

- [x] Fix approach determined (field check on disagreed properties; value-free metamodel messages; per-path read gate on git status and sync)
- [x] Regression test planned
- [x] Related areas checked for similar issues (PATCH warnings leak through the same messages; fixed at the shared point)
