---
id: TKT-I9QKZGH
type: ticket
title: Lua rela.tx on the CreateBatch engine
kind: enhancement
priority: medium
effort: m
status: backlog
---

## Description

Add a Lua `rela.tx(fn)` helper on the `CreateBatch` engine, as DEC-8UIL0 planned
(RES-QKREXZZ ticket 7). Every binding that does external I/O (AI, HTTP, mail,
oauth, attachments) must refuse inside the block through `store.InTx`.
