---
id: BUGA-J7DJN8X
type: bug-analysis-checklist
title: 'Analysis: Flow step that no branch accepts stays put without a message'
started: "2026-11-19"
completed: "2026-11-19"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Reproduction

- [x] Bug reproduced locally (TestFlowControl_CompleteStep "holder, no branch applies" fails on develop: the answer has no problem; the card test "warns when no condition of the next step holds" shows no warning)
- [x] Minimal reproduction steps documented (complete a step whose only change branch has a `when` that does not hold for the entered values)
- [x] Environment/conditions noted (any backend; the person holds the branch role, so the branch is open to them, but its guard is false)

## Root Cause

- [x] Immediate cause identified (why1)
- [x] Contributing factors found (why2-3)
- [x] Systemic cause explored (why4-5)

## Fix Planning

- [x] Fix approach determined (the engine reports Blocked when a change branch was open to the caller but no guard held; FlowControl answers problem `blocked`; the run card shows it as a warning and keeps the saved values)
- [x] Regression test planned (TestFlowControl_CompleteStep, TestEngine_Change, FlowRunCard "warns when no condition of the next step holds")
- [x] Related areas checked for similar issues (the MCP complete_flow_step tool shares FlowControl.Complete and gets the same problem value; a caller who holds no branch role gets no `blocked`, since no branch was open to them)
