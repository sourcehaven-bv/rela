---
id: BUG-KK1UXH
type: bug
title: History restore of a deleted entity fails when its status is past the entry state
description: RecreateEntity enforces the state-machine entry rule, so restoring a deleted entity whose status is not the entry value returns ErrIllegalEntry (422).
priority: medium
status: backlog
---

## Problem

`entitymanager.RecreateEntity` (history restore of a deleted face) calls
`Transitions.EnforceCreate`. A deleted `done` ticket cannot be restored: the
restore returns `ErrIllegalEntry`, a 422 over HTTP and an error from `rela
restore`.

The rule came from sync, where a create carried a peer's new transitions. Sync
is removed (TKT-UA1W1L), so a restore is now the only caller. A restore brings
back recorded history, not an entry transition.

## Expected

Decide whether restore is exempt from the entry rule. If it is, skip
`EnforceCreate` in `RecreateEntity` and invert
`TestTransition_RecreateEntity_EnforcesEntry`. Consider whether a principal
without the right to reach that state by transition may restore it.

## Status (2026-10-02)

Deferred by the owner during TKT-7IZHP0: restore of a deleted entity, including
which face a version snapshot captured, needs its own design before a fix. It
stays in the backlog.
