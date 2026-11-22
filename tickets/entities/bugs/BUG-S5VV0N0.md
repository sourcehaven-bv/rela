---
id: BUG-S5VV0N0
type: bug
title: Flow run list shows a run whose job failed as Working
description: The run list reads the status from the run header; the run page reads it from the run store. A run whose job step failed for good reads Failed on its page and Working in the list.
priority: medium
effort: s
why1: RunList shows the status stored on the run header; the run page shows the status the engine explains from the stored run.
why2: 'A job that fails for good leaves its header at working on purpose: the run stays in its state and a retry can move it on. Only the explanation reads it as failed.'
why3: RunList reads headers only so its cost does not grow with the runs; it never asked the run store about the unfinished runs on its page.
why4: The job failure status was added to the explanation (TKT-PY2G11W) without a test that compares the list with the detail.
why5: Two views of one run take their status from two sources and nothing pins that they agree.
prevention: TestFlowJob_RunListShowsFailedJob compares the list status with the run detail for a failed job; TestFailedJobs_AgreesWithExplain pins the stored-state check the list uses.
started: "2026-11-19"
completed: "2026-11-19"
status: done
---

## Description

The flow run list shows a run whose job step failed for good (RUN-CNPM on
ACC-005 in the harbourmaster portal test) as "Working". The run page of the same
run says "Failed".

## Expected

The list shows the same status as the run page.
