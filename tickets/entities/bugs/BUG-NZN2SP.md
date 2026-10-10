---
id: BUG-NZN2SP
type: bug
title: 'Conflict resolution ignores the fields: write policy'
description: 'authorizeConflictResolve checks only the update ACL. A conflict resolution writes the whole entity file, so it can set a field that fields: makes read-only. Found in the review of BUG-1N7OU9.'
priority: medium
effort: s
status: backlog
---

## Description

`authorizeConflictResolve` (`internal/dataentry/write_handler.go`) re-authorizes
a conflict resolution with the `update` ACL only. Conflict resolution bypasses
entitymanager and writes the whole entity file, so it never reaches
`validateFieldWrite`. A principal with `update` can resolve a conflict with a
version that changes a field the `fields:` policy makes read-only.

## Fix

Diff the resolved entity against the stored one and run `validateFieldWrite` on
the changed and removed properties before the write, as PATCH does.
