---
id: BUGA-P257Z2
type: bug-analysis-checklist
title: 'Analysis: Job queue Close returns before running handlers stop; log-capture tests leave log output on a dead buffer'
started: "2026-10-18"
completed: "2026-10-18"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Reproduction

- [x] Bug reproduced locally (CI sequence replayed by a probe test: 23 of 100 separate `-race -test.cpu=1` runs report the CI race on develop; the natural failure did not show in 20 full-package runs)
- [x] Minimal reproduction steps documented (bug description, Reproduction)
- [x] Environment/conditions noted (needs a job that outlives `Services.Close` and a later log capture; a loaded runner widens the window)

## Root Cause

- [x] Immediate cause identified (why1)
- [x] Contributing factors found (why2-3)
- [x] Systemic cause explored (why4-5)

## Fix Planning

- [x] Fix approach determined (Close waits for running dispatches; shared log-capture helper with full restore)
- [x] Regression test planned (jobstest CloseWaitsForRunningHandlers, memqueue CloseGivesUpOnAStuckHandler, TestCaptureLogs_RestoresLogOutput)
- [x] Related areas checked for similar issues (all appbuild log captures moved to the helper; about 20 captures in other packages keep the incomplete restore, listed in prevention)
