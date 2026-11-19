---
id: BUGA-04RUDEG
type: bug-analysis-checklist
title: 'Analysis: Flow run lists show subject ids and unstyled links'
started: "2026-11-19"
completed: "2026-11-19"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Reproduction

- [x] Bug reproduced locally (TestFlowControl_SubjectTitle and FlowLists.test.ts fail on develop 43e25afdf: runs and tasks carry no subject title, so the tables print the address)
- [x] Minimal reproduction steps documented (start a run on a subject with a display title; open Flow runs or Flow tasks; the Item column shows the id and the links are blue and underlined)
- [x] Environment/conditions noted (any backend; seen in the harbourmaster back office)

## Root Cause

- [x] Immediate cause identified (why1)
- [x] Contributing factors found (why2-3)
- [x] Systemic cause explored (why4-5)

## Fix Planning

- [x] Fix approach determined (add subject_title to the run and task views, read in one gated header batch per page; show it in the tables and on the run page; give the table links the app link class)
- [x] Regression test planned (TestFlowControl_SubjectTitle, TestFlowControl_SubjectTitleHidden, TestWholeDisplayTitle, FlowLists.test.ts)
- [x] Related areas checked for similar issues (the subject panel on an entity page needs no title; dataentry already had two copies of the whole-title rule, now one helper in metamodel)
