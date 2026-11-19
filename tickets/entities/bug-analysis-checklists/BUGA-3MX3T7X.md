---
id: BUGA-3MX3T7X
type: bug-analysis-checklist
title: 'Analysis: A finished run loses its timeline and manage view'
started: "2026-11-19"
completed: "2026-11-19"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Reproduction

- [x] Bug reproduced locally (TestFlowControl_FinishedRunKeepsTimeline fails on develop: no log and no manage action on a finished run)
- [x] Minimal reproduction steps documented (start a run, complete or cancel it, open the run page as a manager)
- [x] Environment/conditions noted (any backend; seen on the harbourmaster portal)

## Root Cause

- [x] Immediate cause identified (why1)
- [x] Contributing factors found (why2-3)
- [x] Systemic cause explored (why4-5)

## Fix Planning

- [x] Fix approach determined (the header keeps the log as its body when the run ends; the run view reads it back on the detail paths and computes the manage action)
- [x] Regression test planned (TestFlowControl_FinishedRunKeepsTimeline and a FlowRunCard test)
- [x] Related areas checked for similar issues (the run list stays a summary without a body read; SubjectRuns gets the timeline like Run)
