---
id: BUGA-19U2QX2
type: bug-analysis-checklist
title: 'Analysis: Flow run page repeats the flow and shows person ids and wrong wording'
started: "2026-11-19"
completed: "2026-11-19"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Reproduction

- [x] Bug reproduced locally (new tests fail on develop: TestFlowControl_TimelineNamesActor has no by_title; flowView shows PSN-5; FlowRunCard shows '1 failed attempts' and 'Waits for no one'; sidebarNav lights the home row)
- [x] Minimal reproduction steps documented (open a run started by a person entity whose job failed once; look at the page title, the card heading, the timeline and the sidebar)
- [x] Environment/conditions noted (any backend; the actor must be an entity id, as in a portal where the principal is a person)

## Root Cause

- [x] Immediate cause identified
- [x] Contributing factors found
- [x] Systemic cause explored

## Fix Planning

- [x] Fix approach determined (the server adds by_title to each event through the gated title lookup it already uses for subjects; the card takes a hideTitle prop on the run page; plural i18n string; the no-roles line is hidden for a failed job; a space home row lights only on the home itself)
- [x] Regression test planned (TestFlowControl_TimelineNamesActor, flowView.test.ts, FlowRunCard.test.ts, sidebarNav.test.ts)
- [x] Related areas checked for similar issues (raw state names come from states without label: in the harbourmaster flows.yaml, which is configuration and not changed here; the subject title was fixed on develop)
