---
id: TKT-MH8VHBV
type: ticket
title: Roll-up where on intermediate hops of a path
kind: enhancement
status: backlog
---

## Description

A path roll-up (TKT-LTSNHD) filters only the far end with `where:`. Entities in
between are filtered only by access. Add a per-hop filter, for example `via:
[{relation: verwijst_naar, where: {status: actief}}, ...]`, compiled into the
step gate query on every backend.
