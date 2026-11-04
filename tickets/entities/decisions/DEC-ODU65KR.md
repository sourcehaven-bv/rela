---
id: DEC-ODU65KR
type: decision
title: 'Atomic batch create: 501 on fs/memory, follow-ups after commit, creates only, fixed limits'
context: 'GitHub #1993 asks for an atomic multi-entity create. RES-QKREXZZ left five questions for the owner.'
consequences: fs and desktop users get a 501 and a documented draft protocol. Automation output is not atomic with the batch in v1. In-Tx automations, rela.tx and an atomic POST /{plural} with relations are follow-ups.
date: "2026-11-04"
status: accepted
---

## Decision

The project owner answered the open questions of RES-QKREXZZ (GitHub #1993):

1. On backends without rollback (fs, memory), `POST /api/v1/_batch` answers
501 `atomic_batch_unsupported`. Support is advertised as
`capabilities.atomic_batch` on `GET /api/v1/_schema`. It is decided once, at
wiring, from the store the deployment runs on.
2. Automations and cascades run after commit, as follow-ups, as a single
create does today. Their results are not part of the atomic unit in v1. A
follow-up failure is reported per operation and does not undo the commit.
3. The surface is `POST /api/v1/_batch`. It only creates entities and
relations: no update or delete. There is no MCP tool in v1.
4. Limits are fixed at 100 operations and a 1 MiB body. They are not
configurable.
5. A batch may create rows the caller cannot read afterwards. The response
redacts them, as a single create does.
