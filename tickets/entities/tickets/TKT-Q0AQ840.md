---
id: TKT-Q0AQ840
type: ticket
title: Automation trigger on entity delete
kind: enhancement
priority: medium
effort: s
started: "2026-11-06"
completed: "2026-11-06"
status: done
description: Add an on.deleted automation trigger that runs background actions with the deleted entity's last state.
---

## Description

Automation triggers are `created`, `updated`, property changes and relation
changes. Nothing fires when an entity is deleted, so a downstream system (a
published site, a search mirror) learns of a delete only by polling.

Add `on: {deleted: true}`. It fires once per removed face, after the delete
commits, for a hard delete, a soft delete, a face delete, an owned cascade
delete and a delete made by a script. A purge of a soft-deleted entity does not
fire again.

- Only `background: true` actions are allowed. A foreground action would write
on behalf of a delete that has already committed.
- `entity` in the script is the deleted face's last stored state, gated and
redacted as the job's identity would read it. A row gate that needs the stored
row (a local role) cannot be judged on a deleted row and fails closed.
- `deleted` cannot be combined with another trigger key; `validate:` and
`related(...)` in a condition are refused.

Source: OpenVWR platform gap R13.

## CI follow-up

`GateDeleted` is a package function in `visibility`, so the script readers keep 20 exported methods (plimsoll). The neoq payload size hint no longer adds to a caller-sized length (CodeQL `go/allocation-size-overflow`).
