---
id: BUGA-R19H45X
type: bug-analysis-checklist
title: 'Analysis: Autosave misjudges its own state across saves of one tab'
started: "2026-10-31"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Reproduction

- [x] Bug reproduced locally (useAutoSave.twoClients.prop.test.ts counterexamples; unit tests in useAutoSave.conflicts.test.ts fail on the old code)
- [x] Minimal reproduction steps documented (each case is a shrunk counterexample, pinned as a unit test: 'keeps a conflicting value on screen', 'keeps a value whose save failed', 'rebases typing onto its own merged save', 'merges a second 412 ... body/relations')
- [x] Environment/conditions noted (two tabs on one entity; a conflict or transient error; typing while a merged save is in flight; a second 412 within one save)

## Root Cause

- [x] Immediate cause identified (why1)
- [x] Contributing factors found (why2-3)
- [x] Systemic cause explored (why4-5)

## Fix Planning

- [x] Fix approach determined (unsavedProps/contentUnsaved held until edit or revert; contentRebase; per-save merge base follows each clean merge, body and relations)
- [x] Regression test planned (two-tab property test plus five unit tests)
- [x] Related areas checked for similar issues (the relations channel had the same second-412 base bug and is fixed here; property merges use propBase and are not affected)
