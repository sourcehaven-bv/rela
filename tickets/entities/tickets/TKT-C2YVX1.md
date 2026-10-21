---
id: TKT-C2YVX1
type: ticket
title: fsstore accepts writes of undeclared types that vanish on reopen
kind: chore
priority: medium
effort: s
status: backlog
description: entityFileKey falls back to type+s for an undeclared type, so the write succeeds, but the scan skips that directory and the entity is gone on reopen.
---

## Description

fsstore accepts a write of an entity whose type the layout does not declare:
`entityFileKey` (internal/store/fsstore/layout.go) falls back to the type name
plus "s" as the directory. The scan on open skips directories that map to no
declared type, so the entity is in the store until the process ends and gone on
every later open. The persisted index cache used to hide this; TKT-33078A
removed the cache, which made the gap visible in `convert_test.go` (it wrote an
undeclared `feature` type).

Found in the code review of TKT-33078A (RR-S3VS4D).

## Fix direction

Reject a write of an undeclared type in fsstore (or earlier, in entitymanager),
with a clear error, so a write cannot succeed and then vanish. Check whether any
write path relies on the fallback (imports, conversions, tests) before removing
it.
