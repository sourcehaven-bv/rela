---
id: BUGA-JS51D6G
type: bug-analysis-checklist
title: 'Analysis: Flow run list shows a run whose job failed as Working'
started: "2026-11-19"
completed: "2026-11-19"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Reproduction

- [x] Bug reproduced locally (TestFlowJob_RunListShowsFailedJob fails on develop: the run detail says failed, the list says working)
- [x] Minimal reproduction steps documented (start a flow whose webhook job answers 400; open the run list and the run page)
- [x] Environment/conditions noted (any backend; a run whose job step failed for good, on the run or on a branch)

## Root Cause

- [x] Immediate cause identified (why1)
- [x] Contributing factors found (why2-3)
- [x] Systemic cause explored (why4-5)

## Fix Planning

- [x] Fix approach determined (RunList reads the stored state of the unfinished runs on the page in one run store read and shows failed for a run that waits on a failed job; the header stays as it is, since a retry can move the run on)
- [x] Regression test planned (TestFlowJob_RunListShowsFailedJob; TestWaitsOnFailedJob for the stored-state check)
- [x] Related areas checked for similar issues (the subject's run panel and the run page use the full explanation and already read failed; the task list filters on input_required and is not affected; the status filter still matches the header, which the flows guide now says)
