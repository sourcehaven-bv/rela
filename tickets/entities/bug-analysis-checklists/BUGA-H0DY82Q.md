---
id: BUGA-H0DY82Q
type: bug-analysis-checklist
title: 'Analysis: Conflict routes answer 500 under a row-dependent read grant on fsstore'
started: "2026-11-05"
completed: "2026-11-05"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Reproduction

- [x] Bug reproduced locally (TestGraphQuery_SkipsConflictedRow fails on origin/develop with "file has unresolved git conflicts")
- [x] Minimal reproduction steps documented (fsstore, two rows of a type, one file rewritten with conflict markers, graph query over the type)
- [x] Environment/conditions noted (fs store only; a read grant with a when: condition)

## Root Cause

- [x] Immediate cause identified (why1)
- [x] Contributing factors found (why2-3)
- [x] Systemic cause explored (why4-5)

## Fix Planning

- [x] Fix approach determined (a graph query leaves out a row whose file is conflicted, so the row gate judges it hidden)
- [x] Regression test planned
- [x] Related areas checked for similar issues (collectByType, collectBranchPrimes and the endpoint match all skip the row; SQL backends have no conflicted rows)
