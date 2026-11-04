---
id: TKT-H46UGLM
type: ticket
title: Make POST /{plural} with relations atomic through CreateBatch
kind: enhancement
priority: medium
effort: s
status: backlog
---

## Description

Route `POST /api/v1/{plural}` with a `relations:` field through
`entitymanager.CreateBatch`, so a single create with edges is atomic on postgres
and sqlite (RES-QKREXZZ ticket 5). Today it calls `CreateEntity` and then writes
the edges, so a failing edge leaves the entity behind.
