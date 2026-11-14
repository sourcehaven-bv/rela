---
id: TKT-OGL697S
type: ticket
title: Push the relation peer gate into the store query
kind: enhancement
priority: low
effort: l
status: backlog
---

## Description

`visibility.ScriptReader.ListRelationsPage` (TKT-P9QMEVI) fills a gated relation
page from raw keyset pages and peer-gates each page in Go. When many edges in
the key range have a hidden endpoint, it reads more raw rows than it serves. On
postgres the gate could run inside the relation query, as the entity row gate
does through `composeReadScope`, so one query returns exactly one page of
visible edges.
