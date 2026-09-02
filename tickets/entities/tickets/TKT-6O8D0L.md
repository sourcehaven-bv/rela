---
id: TKT-6O8D0L
type: ticket
title: Audit rejected attachment uploads (CONTROL-8-15)
kind: enhancement
priority: medium
effort: s
status: done
---

## Description

An upload rejected by the attachment processor — disallowed MIME type, or a
failed/positive scan — returns HTTP 422 and writes **no audit record**.
`handlers_attachment.go`'s `attachment.ErrRejected` branch formats the error and
returns.

A rejected upload is a security-relevant exception: it may be an attempt to
place a disallowed file type or malware into the project. CONTROL-8-15 requires
such events to be logged.

GitHub issue #1050, from IB-review rela#1026. Severity: low.

## Note on the existing coverage

The neighbouring **ACL** denial on the same handler *is* audited
(`handlers_attachment.go`, `OpDeniedWrite` with rule_kind/rule_id/reason). So
the gap is specifically the processor-policy rejection, and the fix should
mirror that record rather than invent a second shape — an operator filtering `op
== "denied-write"` should see both kinds of refused upload.

## Minimum fields

Time, principal, entity + property, filename, and the rejection reason.
