---
id: BUGA-CEYLMZL
type: bug-analysis-checklist
title: 'Analysis: False concurrent-edit warning when one user edits a body inline'
started: "2026-10-30"
completed: "2026-10-30"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Reproduction

- [x] Bug reproduced locally (`EntityDetail.bodyConflict.test.ts` against the fake API shows the production warning)
- [x] Minimal reproduction steps documented (save v1; echo starts a reload; save v2 sent and answered before the reload; reload lands; save v3 gets 412)
- [x] Environment/conditions noted (one user; live events on; typing again within the 250 ms echo delay plus the 800 ms debounce)

## Root Cause

- [x] Immediate cause identified (why1)
- [x] Contributing factors found (why2-3)
- [x] Systemic cause explored (why4-5)

## Fix Planning

- [x] Fix approach determined (own-write tokens in useAutoSave; useEntitySession for EntityDetail)
- [x] Regression test planned (bodyConflict test; session property test; AM-autosave-single-user-no-conflict)
- [x] Related areas checked for similar issues (DynamicForm has the same gap: TKT-60R64HZ; refreshView misses a save that starts and ends during its fetch)
