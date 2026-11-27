---
id: BUGA-TVKE2OZ
type: bug-analysis-checklist
title: 'Analysis: TestOutputs_TriggerFinalCloseOnly misses the close when a mark arrives mid-round'
started: "2026-11-27"
completed: "2026-11-27"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Reproduction

- [x] Bug reproduced locally (a new unit test fails on develop; a 3s sleep after each round makes the integration test fail as in CI)
- [x] Minimal reproduction steps documented (TestWorker_OutputsWakeDuringRoundReruns)
- [x] Environment/conditions noted (the second trigger mark lands while the first outputs round runs)

## Root Cause

- [x] Immediate cause identified (why1)
- [x] Contributing factors found (why2-3)
- [x] Systemic cause explored (why4-5)

## Fix Planning

- [x] Fix approach determined (dirty flag; the round goroutine reruns while it is set)
- [x] Regression test planned
- [x] Related areas checked for similar issues (no other busy-skip pattern in internal/; other worker rounds run inline in the leader loop)
