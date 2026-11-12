---
id: TKT-0FMYPEZ
type: ticket
title: Remote sync status, conflict list and guides
kind: enhancement
priority: medium
effort: m
started: "2026-11-12"
completed: "2026-11-12"
status: done
---

## Description

Operations and docs for remote sync:

- Status per remote: last run, lag, counts, errors, push on/off, conflict
rule, superseded_retention.
- A list of conflicts with resolve and dismiss actions (the git `_conflicts`
view is the model), gated per entity and field for the reader.
- An operator path for uncertain creates.
- Guides in `docs-project/` for setting up a remote and for the two-way case.
The hand-written `docs/remote-sync.md` moves into a guide.

## Acceptance

- Status and conflicts are visible in the data-entry app for operators.
- The guides are generated into `docs/`.

## Notes

- The status values remotesync, the scheduler and the remote routes share
live in the leaf package `internal/remotestatus`. A remote's shown settings
are one struct, read inline from remotes.yaml, so the routes do not import
remotesync and no second type repeats its fields.
