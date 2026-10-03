---
id: TKT-TBGFP2
type: ticket
title: Retry CAS loss in the handler for PATCHes without preconditions
kind: enhancement
priority: medium
effort: s
status: backlog
---

## Description

Since TKT-34XS2R every PATCH that changes an entity is a store compare-and-swap
against the handler's own read. A concurrent write to a different field between
that read and the write makes the store reject it, and the handler answers 412
"concurrent write detected". Writers that send no preconditions (Kanban drag,
list bulk-set, calendar drag, hosted apps through the bridge) have no handling
for that and surface an error for a write that would have been safe: a sparse
patch only names its own fields.

## Approach

For a PATCH that carries no `If-Match` and no `preconditions`, the handler
retries a CAS loss internally (bounded, re-read then re-apply), since
re-applying the same sparse patch is exactly today's semantics. A PATCH with
preconditions keeps returning 412 so the client's merge-and-retry decides
(TKT-2VDVHF).
