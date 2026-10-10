---
id: BUGA-BMZTA6
type: bug-analysis-checklist
title: 'Analysis: Version sweep starves: edits after the first Batch settled rows are never captured'
started: "2026-10-08"
completed: "2026-10-08"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Reproduction

- [x] Bug reproduced locally (storetest `SweepBacklog` against the old queries: pgstore fails all three cases, 299 rows never captured; sqlitestore fails the edit case)
- [x] Minimal reproduction steps documented (250 entities and 249 relations, sweep batch 100; edit the last row after draining)
- [x] Environment/conditions noted (atlas, postgres, 825 entities against the default batch of 500; observed 2026-10-08)

## Root Cause

- [x] Immediate cause identified (why1)
- [x] Contributing factors found (why2-3)
- [x] Systemic cause explored (why4-5)

## Fix Planning

- [x] Fix approach determined (stored `content_hash` on live rows, written back by the sweep and cleared by a trigger; the candidate query compares it with the latest version's hash. A first column-comparison gate was replaced after review, see RR-BB01GI)
- [x] Regression test planned (`storetest.RunSweepBacklogTests`, run by both database backends)
- [x] Related areas checked for similar issues (relation sweep on both backends has the same shape; fixed too)
