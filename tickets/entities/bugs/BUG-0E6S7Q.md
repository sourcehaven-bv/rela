---
id: BUG-0E6S7Q
type: bug
title: Conflict resolve writes without a read gate
description: Conflict resolve takes the type from the raw store and writes without a read gate. It can write to a hidden row and confirm that the row exists.
priority: medium
effort: s
status: backlog
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
