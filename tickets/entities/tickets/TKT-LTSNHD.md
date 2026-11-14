---
id: TKT-LTSNHD
type: ticket
title: Multi-hop roll-up via
kind: enhancement
priority: medium
effort: l
started: "2026-11-13"
completed: "2026-11-14"
status: done
---

## Description

Follow-up of TKT-6I2OPG. A roll-up's `via` follows one relation. GRC projects
need two or three: "is this compliance requirement covered by a measure?" walks
requirement → ISO control → measure, and counting evidence per framework has the
same shape.

Let `via` name a path of relations (`via: [verwijst_naar,
wordt_geimplementeerd_door]`), with every function and `where:` applied to the
far end. Distinct far entities count once per source. The reader's read gate
applies at every hop: an intermediate entity the reader cannot read breaks the
path. The path is walked inside the store's grouped statement (pgstore,
sqlitestore) or by the generic fold (fsstore, memstore), one store call per
column per page.

`avg` stays out of scope (TKT-A2SOOZZ). `where:` on intermediate hops is
deferred (TKT-MH8VHBV).
