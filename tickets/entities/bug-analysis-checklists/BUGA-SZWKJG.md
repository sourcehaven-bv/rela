---
id: BUGA-SZWKJG
type: bug-analysis-checklist
title: 'Analysis: History restore of a deleted entity fails when its status is past the entry state'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Reproduction

- [x] Bug reproduced locally (TestTransition_FacedRestorePastEntry and TestHistoryRestore_DeletedFacePastEntryState fail with ErrIllegalEntry / 422 before the fix)
- [x] Minimal reproduction steps documented (delete a face whose status is past `initial`, then restore its last version)
- [x] Environment/conditions noted (any backend; any type with a state machine; restore of a DELETED entity only)

## Root Cause

- [x] Immediate cause identified (why1)
- [x] Contributing factors found (why2-3)
- [x] Systemic cause explored (why4-5)

## Fix Planning

- [x] Fix approach determined (drop EnforceCreate from RecreateEntity; pin its callers with an archguard test)
- [x] Regression test planned (entitymanager faced + faceless, HTTP restore, ordinary create still refused)
- [x] Related areas checked for similar issues (restore onto a LIVE entity goes through UpdateEntity and keeps transition enforcement, which is correct for a change of an existing record; importer and cascade paths unchanged)
