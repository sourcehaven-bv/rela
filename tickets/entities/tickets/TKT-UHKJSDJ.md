---
id: TKT-UHKJSDJ
type: ticket
title: Export channels with their messages and files
kind: enhancement
priority: medium
status: backlog
---

## Description

Channel messages, their audit trail and, since TKT-BYKOM3V, their files are in
no machine-readable export. `rela export` exports entities only, and `rela db
importfs` copies none of the `channel_*` tables (see TKT-DD8B6I).

Add an export of a project's channels that a customer can use to move to another
system: per channel its key, messages (with properties, authors and times), the
message files and the audit trail, on every backend.

## Acceptance criteria

- One command exports every channel the caller may read, with files.
- The output is documented and stable enough to import elsewhere.
- Works on fs, sqlite and postgres.
