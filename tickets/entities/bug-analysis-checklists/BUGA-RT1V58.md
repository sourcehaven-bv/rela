---
id: BUGA-RT1V58
type: bug-analysis-checklist
title: 'Analysis: TestConvert_RefusesToCopyAnUnreadableField/no_value flakes: map order runs Convert before Preview'
started: "2026-10-18"
completed: "2026-10-18"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Reproduction

- [x] Bug reproduced locally (64 of 500 runs fail on develop 59f68c58e)
- [x] Minimal reproduction steps documented (`go test -race -run TestConvert_RefusesToCopyAnUnreadableField -count=500 ./internal/entitymanager`)
- [x] Environment/conditions noted (any machine; depends only on Go map iteration order)

## Root Cause

- [x] Immediate cause identified (why1)
- [x] Contributing factors found (why2-3)
- [x] Systemic cause explored (why4-5)

## Fix Planning

- [x] Fix approach determined (ordered slice, Preview before Convert)
- [x] Regression test planned (the existing subtest is the regression test; 0 of 500 runs fail with the fix)
- [x] Related areas checked for similar issues (entitymanager tests and the test files changed in PR #1895 and #1907; no other map of calls mutates shared state)
