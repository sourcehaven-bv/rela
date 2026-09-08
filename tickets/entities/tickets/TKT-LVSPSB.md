---
id: TKT-LVSPSB
type: ticket
title: Audit history:read-redacted reveals
kind: enhancement
priority: high
effort: s
status: done
---

## Description

`acl.PermHistoryReadRedacted` ("history:read-redacted") lets an auditor read
frozen historical field values that are deliberately redacted for ordinary
readers — the salary/PII class from TKT-73C6B2. The reveal happens at
`internal/dataentry/history_handler.go:224`, which calls
`serializer.forWireHistoricalReveal` instead of the ordinary `forWire`.

Nothing records that it happened. `internal/audit` logs writes; this is a read.
So "who looked at which hidden historical values, and when" is unanswerable
after the fact — which is the whole point of granting the permission to a small
audited group rather than to everyone.

GitHub issue #1238. Source: rela#1236 (IB-review).

**Violated requirement**: CONTROL-8-15 — logs recording activities, exceptions,
faults and other relevant events shall be produced, stored, protected and
analysed.

## Current state

`serveHistoryVersion` branches on `readGateFromContext(ctx).HoldsPermission(ctx,
acl.PermHistoryReadRedacted)` and emits no audit record on either arm.

The precedent for the fix already exists: `audit.OpACLBypassRead`
(`internal/audit/elevatedread.go`) records that an elevated automation closure
performed a raw read. Its doc comment settles the two design questions this
ticket faces:

- a read-disclosure op is kept SEPARATE from the write op, because folding them
together silently changes the meaning of every existing query for the write op;
- the record carries NO disclosed content, because copying ACL-protected values
into the audit log is a wider disclosure than the read being recorded.

## Scope

IN: an audit record on the reveal arm of entity history version reads.

OUT: relation history. There is deliberately no `history:read-redacted` reveal
for relation history — a deleted relation's meta is served to nobody, and a live
one is redacted against today's policy (see the comment above
`serveRelationHistoryVersion`). No reveal, nothing to log.

OUT: auditing ordinary (non-reveal) history reads. The permission is what makes
this disclosure notable; logging every history read is a different, much larger
decision about read-auditing in general.
