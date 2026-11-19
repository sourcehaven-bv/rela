---
id: BUGA-XMERZLB
type: bug-analysis-checklist
title: 'Analysis: Flows panel offers Start without the start permission'
started: "2026-11-19"
completed: "2026-11-19"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Reproduction

- [x] Bug reproduced locally (FlowRunsPanel.test.ts "offers Start only for a flow the server says the reader may start" fails on the panel of PR #2451; TestFlowControl_Startable shows bob holds no onboarding.start and Start answers ErrForbidden)
- [x] Minimal reproduction steps documented (sign in as a user without the start permission; open an entity whose type has a manual flow; Start shows; clicking it gives 403 "needs permission [...]")
- [x] Environment/conditions noted (any backend; based on PR #2451, which reworks the same panel)

## Root Cause

- [x] Immediate cause identified (why1)
- [x] Contributing factors found (why2-3)
- [x] Systemic cause explored (why4-5)

## Fix Planning

- [x] Fix approach determined (GET /_flows/runs?subject= also answers `startable`, the flows whose manual start permission the caller holds on the entity, checked as Start checks it; the panel offers Start only for those, and none when the runs fail to load)
- [x] Regression test planned (TestFlowControl_Startable, TestFlowsAPI_SubjectRunsStartable, FlowRunsPanel.test.ts)
- [x] Related areas checked for similar issues (Cancel and Retry on a run already follow the run's `actions` from the server; the flow list pages offer no Start)
