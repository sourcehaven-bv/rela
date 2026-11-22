---
id: BUGA-4LRMSEK
type: bug-analysis-checklist
title: 'Analysis: Finished flow run page hides its subject'
started: "2026-11-19"
completed: "2026-11-19"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Reproduction

- [x] Bug reproduced locally (TestFlowControl_FinishedRunNamesSubject fails on develop a856aec54: a cancelled run's detail has an empty subject)
- [x] Minimal reproduction steps documented (start a run, cancel it, GET /api/v1/_flows/runs/{id}: no `subject`; the run page then says "Not visible to you")
- [x] Environment/conditions noted (any backend; the harbourmaster database had nine runs, eight of them completed or cancelled)

## Root Cause

- [x] Immediate cause identified (why1)
- [x] Contributing factors found (why2-3)
- [x] Systemic cause explored (why4-5)

## Fix Planning

- [x] Fix approach determined (read the subject of each header-only run from the gated `_carries` edge, in one query per call, as RunList does)
- [x] Regression test planned (TestFlowControl_FinishedRunNamesSubject: a reader with a global role and one with another role see the subject; a non-reader still gets not found)
- [x] Related areas checked for similar issues (RunList already reads the edge; SubjectRuns shares views and is fixed with it; MCP show_flow_run calls Run; the staff group role and the run gate were checked and admit the run, so staff were not denied the run itself)
