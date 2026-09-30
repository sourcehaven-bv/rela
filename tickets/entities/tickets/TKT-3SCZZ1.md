---
id: TKT-3SCZZ1
type: ticket
title: Soft delete with restore for the Undo toast
kind: enhancement
priority: medium
effort: l
status: done
---

## Description

Back the data-entry Undo toast with a server-side soft delete.

- A web `DELETE` of a whole entity hides the entity and its relations at once,
in every read, search and event feed. The store keeps the rows aside.
- `POST /api/v1/{plural}/{id}/restore` brings them back unchanged. It needs the
delete grants the delete needed, on the entity and on each relation, and the
caller must be able to read the entity or be the user who deleted it.
- A background job on the jobs queue purges the rows once the undo window has
passed (`RELA_SOFT_DELETE_DELAY`, 60s by default).
- While an entity waits, it keeps its id and its `unique:` values.
- A hard delete or rename of the other end of a hidden relation drops that
relation, so a restore cannot attach it to a gone or reused id.

All four stores implement the capability and pass the storetest conformance
suite. pg and sqlite keep the waiting rows in `marked_entities` and
`marked_relations` side tables.
