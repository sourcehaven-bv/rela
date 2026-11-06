---
id: BUG-0E6S7Q
type: bug
title: Conflict resolve writes without a read gate
description: Conflict resolve takes the type from the raw store and writes without a read gate. It can write to a hidden row and confirm that the row exists.
priority: medium
effort: s
why1: Conflict resolve authorized the write with the raw write check and never asked the read gate.
why2: Resolve writes the file directly because the store cannot parse a conflicted file, so it skipped the entitymanager path that runs the read gate first.
why3: The handler rebuilt the ACL question itself instead of asking entitymanager.Manager.AuthorizeUpdate, which runs the read gate and then the write check.
why4: No shared rule said that every served write must pass the read gate before the write check; each write route chose its own authorization.
why5: Write routes that bypass entitymanager have no structural check tying them to the read gate, so a new direct writer can omit it unnoticed.
prevention: Entity resolves authorize through entitymanager.Manager.AuthorizeUpdate after the conflict read gate; a regression test pins the 404 for a hidden row.
started: "2026-11-04"
completed: "2026-11-05"
status: done
---

## Description

The conflict-resolve handler (internal/dataentry/write_handler.go:1596)
authorizes the write itself, but takes the type from the raw `storedTypeOf` and
writes the file directly through `conflict.WriteResolved`. A caller with the
write grant can resolve a conflict on a row they cannot read, and the response
shows that the row exists.

## Fix

Resolve the target through the read gate (`writeTargetOr404`) before
authorizing.

Found during TKT-NXRFDU.
