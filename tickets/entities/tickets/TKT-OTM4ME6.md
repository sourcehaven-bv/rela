---
id: TKT-OTM4ME6
type: ticket
title: 'Public atomic create of several entities and their relations (GitHub #1993)'
kind: enhancement
priority: medium
effort: l
status: wont-fix
---

## Description

GitHub issue #1993 asks for a public operation that creates several entities and
the relations between them as one unit: on success every requested entity and
edge is visible, on failure none is. Today a client needs several REST calls (or
several Lua `rela.create_entity` / `rela.create_relation` calls), and a failure
or disconnect between them leaves a partial graph. The data-entry API reference
lists cross-entity atomic transactions as out of scope.

The owner chose "research first". The research entity linked through
`has-research` surveys the options and proposes a ticket breakdown. No
implementation is planned under this ticket until the owner has decided the open
questions recorded there.

Constraints from the issue and from DEC-8UIL0:

- Use `store.Store.Tx` directly; no unit-of-work or repository layer above it.
- Only postgres and sqlite roll back. fs and memory only serialize, so the API
must not claim all-or-nothing writes there.
- Audit records, store and search events, live delivery and queued jobs must
not survive a rolled-back write.
- Start with a small surface: entity creates plus relation creates, bounded in
size.

## Closed as wont-fix

Shipped as `POST /api/v1/_batch` (2e2c65f46), backed by `entitymanager.CreateBatch`. The research is bd8534c23. Remaining gaps are TKT-H46UGLM and TKT-41YLNDP.
