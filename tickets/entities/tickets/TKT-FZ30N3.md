---
id: TKT-FZ30N3
type: ticket
title: Pin the write-ACL refusal of relations a conversion removes
kind: test
priority: low
effort: xs
started: "2026-10-22"
completed: "2026-10-22"
status: done
---

## Description

GitHub #1900 reported that a conversion drops and remaps relations without a
relation audit record, a relation version or a write-ACL check. Commit 563989583
(PR #1895) already does all three: `conversionRun.authorizeRemovals` checks
every removed relation through `authorizeCascadeRelations`, and
`conversionRun.record` writes the audit records and delete versions.

Audit and versioning are pinned by
`TestConvert_RelationsAreDroppedAndRemappedWithRecords`. The write-ACL refusal
has no test. This ticket adds one: a conversion that must drop an incoming
relation from a type the caller cannot write is refused with `acl.ErrForbidden`,
writes a `denied-write` audit record, and leaves the relation stored.
