---
id: BUGA-EVMDB1J
type: bug-analysis-checklist
title: 'Analysis: TestSSEACL tests flake on a fixed settle() sleep'
started: "2026-11-27"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Reproduction

- [x] ~~Bug reproduced locally~~ (not reproduced: the flake needs a loaded CI runner; cause confirmed from the CI log and the code)
- [x] Minimal reproduction steps documented (ticket body: merge-queue run 43582373554 on #2256)
- [x] Environment/conditions noted (loaded CI runner; the flush lands after the fixed sleep)

## Root Cause

- [x] Immediate cause identified (why1)
- [x] Contributing factors found (why2-3)
- [x] Systemic cause explored (why4-5)

## Fix Planning

- [x] Fix approach determined (wait until a ping frame's resume token reaches the last feed position sent)
- [x] Regression test planned (the TestSSEACL suite itself: a -race -count=50 stress run plus a leaking-gate mutation check)
- [x] Related areas checked for similar issues (sse_resume_test.go and live_served_test.go have no fixed sleeps)
