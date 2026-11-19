---
id: BUG-TVU9HRT
type: bug
title: Finished flow run page hides its subject
description: 'The page of a finished flow run says Not visible to you for every reader who may read the subject: the run detail drops the subject address of a run that keeps only its header.'
priority: medium
effort: s
why1: The run detail sets the subject only for a run with stored working state; a finished run keeps only its header, so its subject was empty and the page showed Not visible to you.
why2: FlowControl.views took the subject from the run store, which drops a run once it finishes, and never read the _carries edge that the run list reads.
why3: The detail path was written for active runs (waiting reason, timeline, step form) and the header-only branch returned the header unchanged.
why4: The run view tests read finished runs only through the list, never through the detail.
why5: Two read paths (list and detail) each derive the subject in their own way, and no test compares them on a finished run.
prevention: TestFlowControl_FinishedRunNamesSubject reads a finished run through Run and SubjectRuns for two readers and a non-reader.
started: "2026-11-19"
status: in-progress
---

## Description

Seen in harbourmaster as staff: `/s/backoffice/flows/runs/RUN-xxxx` says "Not
visible to you" and shows no steps or timeline, although staff read the subject
account. Every run in that database except one had finished (completed or
cancelled).
