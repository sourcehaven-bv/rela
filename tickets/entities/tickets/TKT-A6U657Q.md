---
id: TKT-A6U657Q
type: ticket
title: 'Remote pull: refuse symlinked credential files and hide unreadable conflict rows'
kind: chore
priority: high
effort: s
started: "2026-11-09"
completed: "2026-11-09"
status: done
---

## Description

Two read-side leaks in the remote pull (TKT-I8L1EQU).

1. A declared credential file is checked by its base name, then opened with `os.Open`, which follows symlinks. A `remote_credential.*` symlink to another file would send that file to the remote. The read now refuses anything that is not a regular file and checks that the opened file is the one it inspected.
2. `applyEntity` locates the local row through the raw store. A conflict reported the row's local id and type even when the bound principal cannot read it. A row the principal cannot read is now reported as a conflict that does not identify it, and the run does not merge into it.

## Follow-up

- A credential file whose hard-link count cannot be read is refused (fail closed). Windows reads the count from the file handle.
- The declared-file check lives in `secrets.OpenRemoteCredentialFile`, next to the naming rule, to satisfy archplan.
