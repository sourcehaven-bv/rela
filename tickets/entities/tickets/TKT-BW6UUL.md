---
id: TKT-BW6UUL
type: ticket
title: 'Operator ''purge version'' primitive: hard-delete a snapshot row for compliance redaction'
kind: enhancement
priority: low
effort: s
status: backlog
---

Follow-up to TKT-9INY0Y / design-review finding RR-A3RNT0.

## Context

pgstore content versioning (TKT-9INY0Y) stores full entity snapshots. History
read is all-or-nothing from creation and is NOT point-in-time-ACL-aware (that's
the IDEA-CQMKMD epic). Consequence: if content is edited out of an entity for
compliance reasons (PII, a rotated secret), an older version snapshot still
contains it, and anyone who can read the entity's history can resurrect it —
including via restore, which makes it live again.

## What this ticket does

Add an operator-only 'purge version' primitive that HARD-DELETES a specific
`entity_versions` snapshot row (and, optionally, all versions of an entity), so
a compliance redaction actually removes the sensitive content from history
rather than leaving it recoverable. This is the deliberate exception to the
append-only history model.

## Considerations

- Gate behind a high privilege (operator/admin), distinct from `history:read`.
- Audit the purge itself (who purged which version and when) — the purge is a forensic event.
- Decide interaction with retention and with the 'restore' path (a purged version can't be restored).
- Likely CLI-only initially (operator context), not a data-entry UI action.
