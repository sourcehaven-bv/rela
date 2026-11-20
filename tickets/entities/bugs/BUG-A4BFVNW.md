---
id: BUG-A4BFVNW
type: bug
title: A finished run loses its timeline and manage view
description: The run log is dropped with the run state when a run ends, and a finished run view never says the reader manages the flow.
priority: medium
effort: s
why1: Engine.finish and Engine.Cancel delete the run state, and the log lived only there; the header kept flow, status, state and times.
why2: The finished branch of the run view returned the header only, with no actions, so the card hid the manage view.
why3: The timeline was built for active runs (TKT-ZAXRXND); the finished run was treated as a summary row in the run list.
why4: No test opened a finished run as a manager.
why5: The run view tests cover each state of an active run but not the end of a run.
prevention: TestFlowControl_FinishedRunKeepsTimeline opens completed and cancelled runs as a manager and as a reader; TestFlowControl_FinishedRunWithoutLog covers headers without a readable log.
started: "2026-11-19"
completed: "2026-11-19"
status: done
---

## Description

In the harbourmaster portal, a manager opens a completed run. The run card shows
its state and status only: no timeline, no diagram, and no manage view. While
the run was active the same manager saw all three.

## Cause

A finished run keeps only its header; the engine deletes its stored state, which
holds the log. The run view of a finished run sets no `manage` action, and the
card shows the timeline and diagram only to managers.

## Expected

A manager sees the timeline and the diagram (at the final state) of a finished
run. A reader who does not manage the flow sees what they see today.
